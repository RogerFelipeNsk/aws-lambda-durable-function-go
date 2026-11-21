package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/catalog"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/config"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/httpapi"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/orchestrator"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/payment"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/store"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/workflow"
)

type guestResp struct {
	Token string `json:"token"`
}

type orderResp struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Pix    *struct {
		CopiaECola string `json:"copiaECola"`
	} `json:"pix"`
}

type productResp struct {
	ID        string `json:"id"`
	Available int    `json:"available"`
	Reserved  int    `json:"reserved"`
	Sold      int    `json:"sold"`
}

func TestCheckoutPixAndConfirm(t *testing.T) {
	handler, st := newServer(t)
	token := login(t, handler)

	code := do(t, handler, http.MethodPost, "/api/cart/items", token, map[string]any{
		"productId": "bola-futebol-pro",
		"quantity":  1,
	})
	if code != http.StatusOK {
		t.Fatalf("add item status %d", code)
	}

	waitOpen(t, handler, token)

	rec := request(t, handler, http.MethodPost, "/api/checkout", token, map[string]any{"method": "pix"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("checkout %d %s", rec.Code, rec.Body.String())
	}
	var order orderResp
	if err := json.Unmarshal(rec.Body.Bytes(), &order); err != nil {
		t.Fatal(err)
	}

	order = waitOrderStatus(t, handler, token, order.ID, "awaiting_pix")
	if order.Pix == nil || order.Pix.CopiaECola == "" {
		t.Fatal("expected PIX payload")
	}

	rec = request(t, handler, http.MethodPost, "/api/orders/"+order.ID+"/pix/confirm", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("confirm %d %s", rec.Code, rec.Body.String())
	}
	order = waitOrderStatus(t, handler, token, order.ID, "paid")

	p := getProduct(t, handler, "bola-futebol-pro")
	if p.Available != 39 || p.Sold != 1 {
		t.Fatalf("stock after pay %+v", p)
	}
	_ = st
}

func TestCheckoutPixExpireRestoresStock(t *testing.T) {
	handler, _ := newServer(t)
	token := login(t, handler)
	do(t, handler, http.MethodPost, "/api/cart/items", token, map[string]any{
		"productId": "bola-basquete",
		"quantity":  2,
	})
	waitOpen(t, handler, token)
	rec := request(t, handler, http.MethodPost, "/api/checkout", token, map[string]any{"method": "pix"})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("checkout %d %s", rec.Code, rec.Body.String())
	}
	var order orderResp
	_ = json.Unmarshal(rec.Body.Bytes(), &order)
	order = waitOrderStatus(t, handler, token, order.ID, "awaiting_pix")

	p := getProduct(t, handler, "bola-basquete")
	if p.Reserved != 2 {
		t.Fatalf("expected reserved 2, got %+v", p)
	}

	rec = request(t, handler, http.MethodPost, "/api/orders/"+order.ID+"/pix/expire", token, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expire %d %s", rec.Code, rec.Body.String())
	}
	_ = waitOrderStatus(t, handler, token, order.ID, "pix_expired")
	p = getProduct(t, handler, "bola-basquete")
	if p.Available != 30 || p.Reserved != 0 || p.Sold != 0 {
		t.Fatalf("stock after expire %+v", p)
	}
}

func TestCheckoutCard(t *testing.T) {
	handler, _ := newServer(t)
	token := login(t, handler)
	do(t, handler, http.MethodPost, "/api/cart/items", token, map[string]any{
		"productId": "squeeze",
		"quantity":  1,
	})
	waitOpen(t, handler, token)
	rec := request(t, handler, http.MethodPost, "/api/checkout", token, map[string]any{
		"method": "card",
		"card": map[string]string{
			"number": payment.TestNumber,
			"exp":    payment.TestExp,
			"cvc":    payment.TestCVC,
		},
	})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("checkout %d %s", rec.Code, rec.Body.String())
	}
	var order orderResp
	_ = json.Unmarshal(rec.Body.Bytes(), &order)
	_ = waitOrderStatus(t, handler, token, order.ID, "paid")
}

func newServer(t *testing.T) (http.Handler, *store.Memory) {
	t.Helper()
	st := store.NewMemory()
	if err := catalog.Seed(t.Context(), st); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		CartTTL:       time.Hour,
		PixTTL:        time.Hour,
		CardDelay:     0,
		CardDelayLong: 0,
		SessionTTL:    time.Hour,
		AllowedOrigin: "*",
	}
	orch := orchestrator.NewLocal(st, cfg, workflow.New(st, cfg))
	return httpapi.New(st, orch, cfg), st
}

func login(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := request(t, h, http.MethodPost, "/api/auth/guest", "", nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("login %d %s", rec.Code, rec.Body.String())
	}
	var g guestResp
	if err := json.Unmarshal(rec.Body.Bytes(), &g); err != nil {
		t.Fatal(err)
	}
	return g.Token
}

func waitOpen(t *testing.T, h http.Handler, token string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		rec := request(t, h, http.MethodGet, "/api/cart", token, nil)
		var cart struct {
			Status string `json:"status"`
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &cart)
		if cart.Status == "open" {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("cart did not become open")
}

func waitOrderStatus(t *testing.T, h http.Handler, token, id, want string) orderResp {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var order orderResp
	for time.Now().Before(deadline) {
		rec := request(t, h, http.MethodGet, "/api/orders/"+id, token, nil)
		_ = json.Unmarshal(rec.Body.Bytes(), &order)
		if order.Status == want {
			return order
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("order status = %s want %s", order.Status, want)
	return order
}

func getProduct(t *testing.T, h http.Handler, id string) productResp {
	t.Helper()
	rec := request(t, h, http.MethodGet, "/api/products/"+id, "", nil)
	var p productResp
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func do(t *testing.T, h http.Handler, method, path, token string, body any) int {
	t.Helper()
	return request(t, h, method, path, token, body).Code
}

func request(t *testing.T, h http.Handler, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}
