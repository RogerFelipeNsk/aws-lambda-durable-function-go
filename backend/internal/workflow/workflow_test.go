package workflow_test

import (
	"testing"
	"time"

	"github.com/aws/aws-durable-execution-sdk-go/durable/durabletest"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/catalog"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/config"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/domain"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/payment"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/store"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/workflow"
)

const productID = "bola-futebol-pro"

func setup(t *testing.T) (*store.Memory, *durabletest.LocalRunner[domain.OrchestratorEvent, domain.OrchestratorResult], domain.OrchestratorEvent) {
	t.Helper()
	st := store.NewMemory()
	if err := catalog.Seed(t.Context(), st); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	cart := domain.Cart{
		ID:        "cart_test",
		UserID:    "usr_test",
		Status:    domain.CartStarting,
		Items:     []domain.CartItem{{ProductID: productID, Name: "Bola", UnitCents: 19990, Quantity: 2}},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := st.PutCart(t.Context(), cart); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Reserve(t.Context(), cart.ID, productID, 2); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		CartTTL:       time.Hour,
		PixTTL:        time.Hour,
		CardDelay:     0,
		CardDelayLong: 0,
	}
	runner := durabletest.NewLocalRunner(workflow.New(st, cfg))
	event := domain.OrchestratorEvent{CartID: cart.ID}
	result, err := runner.RunUntilComplete(event)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != durabletest.Pending {
		t.Fatalf("status = %s, want PENDING", result.Status)
	}
	return st, runner, event
}

func TestPixPaidCommitsStock(t *testing.T) {
	st, runner, event := setup(t)
	ctx := t.Context()
	order := placeOrder(t, st, "pix")
	cb := mustCallback(t, runner)
	if err := runner.SendCallbackSuccess(cb, domain.CheckoutSignal{OrderID: order.ID, Method: domain.PayPix}); err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunUntilComplete(event)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != durabletest.Pending {
		t.Fatalf("after checkout status = %s", result.Status)
	}
	pixCB := mustCallback(t, runner)
	if err := runner.SendCallbackSuccess(pixCB, domain.PixSignal{Paid: true}); err != nil {
		t.Fatal(err)
	}
	result, err = runner.RunUntilComplete(event)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != durabletest.Succeeded {
		t.Fatalf("status = %s err=%v", result.Status, result.Error)
	}
	p, _ := st.GetProduct(ctx, productID)
	if p.Available != 38 || p.Reserved != 0 || p.Sold != 2 {
		t.Fatalf("stock %+v", p)
	}
	got, _ := st.GetOrder(ctx, order.ID)
	if got.Status != domain.OrderPaid {
		t.Fatalf("order status %s", got.Status)
	}
}

func TestPixExpiredRestoresStock(t *testing.T) {
	st, runner, event := setup(t)
	ctx := t.Context()
	order := placeOrder(t, st, "pix")
	if err := runner.SendCallbackSuccess(mustCallback(t, runner), domain.CheckoutSignal{OrderID: order.ID, Method: domain.PayPix}); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.RunUntilComplete(event); err != nil {
		t.Fatal(err)
	}
	if err := runner.TimeoutCallback(mustCallback(t, runner)); err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunUntilComplete(event)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != durabletest.Succeeded {
		t.Fatalf("status = %s err=%v", result.Status, result.Error)
	}
	p, _ := st.GetProduct(ctx, productID)
	if p.Available != 40 || p.Reserved != 0 || p.Sold != 0 {
		t.Fatalf("stock not restored: %+v", p)
	}
	got, _ := st.GetOrder(ctx, order.ID)
	if got.Status != domain.OrderPixExpired {
		t.Fatalf("order status %s", got.Status)
	}
}

func TestCartExpiredRestoresStock(t *testing.T) {
	st, runner, event := setup(t)
	if err := runner.TimeoutCallback(mustCallback(t, runner)); err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunUntilComplete(event)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != durabletest.Succeeded {
		t.Fatalf("status = %s err=%v", result.Status, result.Error)
	}
	p, _ := st.GetProduct(t.Context(), productID)
	if p.Available != 40 || p.Reserved != 0 {
		t.Fatalf("stock not restored: %+v", p)
	}
	cart, _ := st.GetCart(t.Context(), event.CartID)
	if cart.Status != domain.CartExpired {
		t.Fatalf("cart status %s", cart.Status)
	}
}

func TestCardPaid(t *testing.T) {
	st, runner, event := setup(t)
	order := placeOrder(t, st, "card")
	signal := domain.CheckoutSignal{
		OrderID: order.ID,
		Method:  domain.PayCard,
		Card:    &domain.CardData{Number: payment.TestNumber, Exp: payment.TestExp, CVC: payment.TestCVC},
	}
	if err := runner.SendCallbackSuccess(mustCallback(t, runner), signal); err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunUntilComplete(event)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != durabletest.Succeeded {
		t.Fatalf("status = %s err=%v", result.Status, result.Error)
	}
	p, _ := st.GetProduct(t.Context(), productID)
	if p.Sold != 2 || p.Available != 38 {
		t.Fatalf("stock %+v", p)
	}
	got, _ := st.GetOrder(t.Context(), order.ID)
	if got.Status != domain.OrderPaid || got.CardLast4 != "1111" {
		t.Fatalf("order %+v", got)
	}
}

func TestCardDeclinedRestoresStock(t *testing.T) {
	st, runner, event := setup(t)
	order := placeOrder(t, st, "card")
	signal := domain.CheckoutSignal{
		OrderID:  order.ID,
		Method:   domain.PayCard,
		Simulate: "card_fail",
		Card:     &domain.CardData{Number: payment.TestNumber, Exp: payment.TestExp, CVC: payment.TestCVC},
	}
	if err := runner.SendCallbackSuccess(mustCallback(t, runner), signal); err != nil {
		t.Fatal(err)
	}
	result, err := runner.RunUntilComplete(event)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != durabletest.Succeeded {
		t.Fatalf("status = %s err=%v", result.Status, result.Error)
	}
	p, _ := st.GetProduct(t.Context(), productID)
	if p.Available != 40 || p.Sold != 0 {
		t.Fatalf("stock %+v", p)
	}
	got, _ := st.GetOrder(t.Context(), order.ID)
	if got.Status != domain.OrderDeclined {
		t.Fatalf("order status %s", got.Status)
	}
}

func placeOrder(t *testing.T, st store.Store, method string) domain.Order {
	t.Helper()
	now := time.Now().UTC()
	order := domain.Order{
		ID:         "ord_test",
		UserID:     "usr_test",
		CartID:     "cart_test",
		Status:     domain.OrderPendingPayment,
		Method:     method,
		Items:      []domain.CartItem{{ProductID: productID, Quantity: 2, UnitCents: 19990}},
		TotalCents: 39980,
		Currency:   domain.CurrencyBRL,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := st.PutOrder(t.Context(), order); err != nil {
		t.Fatal(err)
	}
	return order
}

func mustCallback(t *testing.T, runner *durabletest.LocalRunner[domain.OrchestratorEvent, domain.OrchestratorResult]) string {
	t.Helper()
	cbs := runner.OpenCallbacks()
	if len(cbs) == 0 {
		t.Fatal("no open callbacks")
	}
	return cbs[0].CallbackID
}
