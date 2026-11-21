package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/config"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/domain"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/guest"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/id"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/orchestrator"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/payment"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/store"
)

type ctxKey int

const userKey ctxKey = 1

type API struct {
	store store.Store
	orch  orchestrator.Orchestrator
	cfg   config.Config
	mux   *http.ServeMux
}

func New(st store.Store, orch orchestrator.Orchestrator, cfg config.Config) http.Handler {
	a := &API{store: st, orch: orch, cfg: cfg, mux: http.NewServeMux()}
	a.mux.HandleFunc("GET /health", a.health)
	a.mux.HandleFunc("GET /api/products", a.listProducts)
	a.mux.HandleFunc("GET /api/products/{id}", a.getProduct)
	a.mux.HandleFunc("POST /api/auth/guest", a.guestLogin)
	a.mux.HandleFunc("GET /api/me", a.auth(a.me))
	a.mux.HandleFunc("GET /api/cart", a.auth(a.getCart))
	a.mux.HandleFunc("POST /api/cart/items", a.auth(a.addItem))
	a.mux.HandleFunc("PATCH /api/cart/items/{id}", a.auth(a.updateItem))
	a.mux.HandleFunc("DELETE /api/cart/items/{id}", a.auth(a.removeItem))
	a.mux.HandleFunc("POST /api/checkout", a.auth(a.checkout))
	a.mux.HandleFunc("GET /api/orders", a.auth(a.listOrders))
	a.mux.HandleFunc("GET /api/orders/{id}", a.auth(a.getOrder))
	a.mux.HandleFunc("POST /api/orders/{id}/pix/confirm", a.auth(a.confirmPix))
	a.mux.HandleFunc("POST /api/orders/{id}/pix/expire", a.auth(a.expirePix))
	return a
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", a.cfg.AllowedOrigin)
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	a.mux.ServeHTTP(w, r)
}

func (a *API) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "arena-store",
		"mode":    a.mode(),
	})
}

func (a *API) mode() string {
	if a.cfg.DurableFunctionName == "" {
		return "local"
	}
	return "aws"
}

func (a *API) listProducts(w http.ResponseWriter, r *http.Request) {
	products, err := a.store.ListProducts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]domain.Product, 0, len(products))
	for _, p := range products {
		out = append(out, p.Public())
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) getProduct(w http.ResponseWriter, r *http.Request) {
	p, err := a.store.GetProduct(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, p.Public())
}

func (a *API) guestLogin(w http.ResponseWriter, r *http.Request) {
	user := guest.NewUser()
	if err := a.store.CreateUser(r.Context(), user); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	session := domain.Session{
		ID:        id.Session(),
		UserID:    user.ID,
		CreatedAt: time.Now().UTC(),
		ExpiresAt: time.Now().UTC().Add(a.cfg.SessionTTL),
	}
	if err := a.store.CreateSession(r.Context(), session); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":     session.ID,
		"user":      user,
		"expiresAt": session.ExpiresAt,
	})
}

func (a *API) me(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, userFrom(r.Context()))
}

func (a *API) getCart(w http.ResponseWriter, r *http.Request) {
	cart, err := a.currentCart(r.Context(), userFrom(r.Context()).ID, false)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeJSON(w, http.StatusOK, domain.Cart{Items: []domain.CartItem{}, Status: "empty"})
			return
		}
		writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cartView(cart))
}

type itemBody struct {
	ProductID string `json:"productId"`
	Quantity  int    `json:"quantity"`
}

func (a *API) addItem(w http.ResponseWriter, r *http.Request) {
	var body itemBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("json inválido"))
		return
	}
	if body.Quantity < 1 {
		body.Quantity = 1
	}
	user := userFrom(r.Context())
	cart, err := a.ensureCart(r.Context(), user)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if cart.Status != domain.CartOpen && cart.Status != domain.CartStarting {
		writeError(w, http.StatusConflict, store.ErrCartClosed)
		return
	}
	product, err := a.store.GetProduct(r.Context(), body.ProductID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if _, err := a.store.Reserve(r.Context(), cart.ID, product.ID, body.Quantity); err != nil {
		writeStoreError(w, err)
		return
	}
	cart, err = a.store.GetCart(r.Context(), cart.ID)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	cart.Items = upsertItem(cart.Items, product, body.Quantity)
	store.AppendCartEvent(&cart, "item_added", product.Name+" x"+strconv.Itoa(body.Quantity))
	if err := a.store.PutCart(r.Context(), cart); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cartView(cart))
}

func (a *API) updateItem(w http.ResponseWriter, r *http.Request) {
	var body itemBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("json inválido"))
		return
	}
	cart, err := a.currentCart(r.Context(), userFrom(r.Context()).ID, true)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if cart.Status != domain.CartOpen && cart.Status != domain.CartStarting {
		writeError(w, http.StatusConflict, store.ErrCartClosed)
		return
	}
	productID := r.PathValue("id")
	current := cart.ItemQty(productID)
	if current == 0 {
		writeError(w, http.StatusNotFound, errors.New("item não está no carrinho"))
		return
	}
	if body.Quantity < 1 {
		a.deleteQty(w, r, cart, productID, current)
		return
	}
	delta := body.Quantity - current
	if delta != 0 {
		if _, err := a.store.Reserve(r.Context(), cart.ID, productID, delta); err != nil {
			writeStoreError(w, err)
			return
		}
	}
	cart.Items = setQty(cart.Items, productID, body.Quantity)
	store.AppendCartEvent(&cart, "item_updated", productID+" → "+strconv.Itoa(body.Quantity))
	if err := a.store.PutCart(r.Context(), cart); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cartView(cart))
}

func (a *API) removeItem(w http.ResponseWriter, r *http.Request) {
	cart, err := a.currentCart(r.Context(), userFrom(r.Context()).ID, true)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if cart.Status != domain.CartOpen && cart.Status != domain.CartStarting {
		writeError(w, http.StatusConflict, store.ErrCartClosed)
		return
	}
	productID := r.PathValue("id")
	current := cart.ItemQty(productID)
	if current == 0 {
		writeError(w, http.StatusNotFound, errors.New("item não está no carrinho"))
		return
	}
	a.deleteQty(w, r, cart, productID, current)
}

func (a *API) deleteQty(w http.ResponseWriter, r *http.Request, cart domain.Cart, productID string, qty int) {
	if _, err := a.store.Reserve(r.Context(), cart.ID, productID, -qty); err != nil {
		writeStoreError(w, err)
		return
	}
	cart.Items = removeItem(cart.Items, productID)
	store.AppendCartEvent(&cart, "item_removed", productID)
	if err := a.store.PutCart(r.Context(), cart); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, cartView(cart))
}

type checkoutBody struct {
	Method   string           `json:"method"`
	Card     *domain.CardData `json:"card"`
	Simulate string           `json:"simulate"`
}

func (a *API) checkout(w http.ResponseWriter, r *http.Request) {
	var body checkoutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("json inválido"))
		return
	}
	if body.Method != domain.PayPix && body.Method != domain.PayCard {
		writeError(w, http.StatusBadRequest, errors.New("método deve ser pix ou card"))
		return
	}
	user := userFrom(r.Context())
	cart, err := a.currentCart(r.Context(), user.ID, true)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if cart.Status != domain.CartOpen {
		writeError(w, http.StatusConflict, errors.New("carrinho ainda não está pronto para checkout"))
		return
	}
	if cart.CheckoutCallbackID == "" {
		writeError(w, http.StatusConflict, errors.New("sessão durable ainda inicializando, tente de novo"))
		return
	}
	if len(cart.Items) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("carrinho vazio"))
		return
	}
	if body.Method == domain.PayCard {
		if body.Card == nil {
			writeError(w, http.StatusBadRequest, errors.New("dados do cartão obrigatórios"))
			return
		}
		if err := payment.Validate(payment.Input{Number: body.Card.Number, Exp: body.Card.Exp, CVC: body.Card.CVC}); err != nil && !errors.Is(err, payment.ErrDeclined) {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	now := time.Now().UTC()
	order := domain.Order{
		ID:         id.New("ord"),
		UserID:     user.ID,
		CartID:     cart.ID,
		Status:     domain.OrderPendingPayment,
		Method:     body.Method,
		Items:      cart.Items,
		TotalCents: cart.TotalCents(),
		Currency:   domain.CurrencyBRL,
		Simulate:   body.Simulate,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	store.AppendOrderEvent(&order, "created", "pedido criado, aguardando orquestração durable")
	if err := a.store.PutOrder(r.Context(), order); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	signal := domain.CheckoutSignal{
		OrderID:  order.ID,
		Method:   body.Method,
		Card:     body.Card,
		Simulate: body.Simulate,
	}
	if err := a.orch.SignalCheckout(r.Context(), cart.CheckoutCallbackID, signal); err != nil {
		slog.Error("signal checkout", "err", err)
		writeError(w, http.StatusBadGateway, errors.New("falha ao sinalizar a função durable"))
		return
	}

	fresh, err := a.store.GetOrder(r.Context(), order.ID)
	if err != nil {
		writeJSON(w, http.StatusAccepted, order)
		return
	}
	writeJSON(w, http.StatusAccepted, fresh)
}

func (a *API) listOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := a.store.ListOrdersByUser(r.Context(), userFrom(r.Context()).ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, orders)
}

func (a *API) getOrder(w http.ResponseWriter, r *http.Request) {
	order, err := a.store.GetOrder(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if order.UserID != userFrom(r.Context()).ID {
		writeError(w, http.StatusNotFound, store.ErrNotFound)
		return
	}
	writeJSON(w, http.StatusOK, order)
}

func (a *API) confirmPix(w http.ResponseWriter, r *http.Request) {
	cart, order, err := a.orderCart(r)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if order.Status != domain.OrderAwaitingPix || cart.PixCallbackID == "" {
		writeError(w, http.StatusConflict, errors.New("pedido não está aguardando PIX"))
		return
	}
	if order.Pix != nil && time.Now().UTC().After(order.Pix.ExpiresAt) {
		writeError(w, http.StatusConflict, errors.New("PIX já expirou"))
		return
	}
	if err := a.orch.SignalPixPaid(r.Context(), cart.PixCallbackID); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	fresh, err := a.store.GetOrder(r.Context(), order.ID)
	if err != nil {
		writeJSON(w, http.StatusAccepted, order)
		return
	}
	writeJSON(w, http.StatusOK, fresh)
}

func (a *API) expirePix(w http.ResponseWriter, r *http.Request) {
	cart, order, err := a.orderCart(r)
	if err != nil {
		writeStoreError(w, err)
		return
	}
	if order.Status != domain.OrderAwaitingPix || cart.PixCallbackID == "" {
		writeError(w, http.StatusConflict, errors.New("pedido não está aguardando PIX"))
		return
	}
	if err := a.orch.SignalFailure(r.Context(), cart.PixCallbackID, "PixExpired", "simulação de expiração PIX"); err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	fresh, err := a.store.GetOrder(r.Context(), order.ID)
	if err != nil {
		writeJSON(w, http.StatusAccepted, order)
		return
	}
	writeJSON(w, http.StatusOK, fresh)
}

func (a *API) orderCart(r *http.Request) (domain.Cart, domain.Order, error) {
	order, err := a.store.GetOrder(r.Context(), r.PathValue("id"))
	if err != nil {
		return domain.Cart{}, domain.Order{}, err
	}
	if order.UserID != userFrom(r.Context()).ID {
		return domain.Cart{}, domain.Order{}, store.ErrNotFound
	}
	cart, err := a.store.GetCart(r.Context(), order.CartID)
	return cart, order, err
}

func (a *API) ensureCart(ctx context.Context, user domain.User) (domain.Cart, error) {
	cart, err := a.store.GetOpenCartByUser(ctx, user.ID)
	if err == nil {
		return cart, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return domain.Cart{}, err
	}
	now := time.Now().UTC()
	cart = domain.Cart{
		ID:        id.New("cart"),
		UserID:    user.ID,
		Status:    domain.CartStarting,
		Items:     []domain.CartItem{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	store.AppendCartEvent(&cart, "created", "carrinho criado, iniciando função durable")
	if err := a.store.PutCart(ctx, cart); err != nil {
		return domain.Cart{}, err
	}
	if _, err := a.orch.StartCartSession(ctx, cart.ID); err != nil {
		return domain.Cart{}, err
	}
	fresh, err := a.store.GetCart(ctx, cart.ID)
	if err != nil {
		return cart, nil
	}
	return fresh, nil
}

func (a *API) currentCart(ctx context.Context, userID string, required bool) (domain.Cart, error) {
	cart, err := a.store.GetOpenCartByUser(ctx, userID)
	if err != nil && required {
		return domain.Cart{}, err
	}
	return cart, err
}

func (a *API) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			writeError(w, http.StatusUnauthorized, errors.New("faça login como visitante"))
			return
		}
		session, err := a.store.GetSession(r.Context(), token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, errors.New("sessão inválida"))
			return
		}
		user, err := a.store.GetUser(r.Context(), session.UserID)
		if err != nil {
			writeError(w, http.StatusUnauthorized, errors.New("usuário não encontrado"))
			return
		}
		next(w, r.WithContext(context.WithValue(r.Context(), userKey, user)))
	}
}

func userFrom(ctx context.Context) domain.User {
	u, _ := ctx.Value(userKey).(domain.User)
	return u
}

type cartResponse struct {
	domain.Cart
	TotalCents int64 `json:"totalCents"`
}

func cartView(c domain.Cart) cartResponse {
	c.CheckoutCallbackID = ""
	c.PixCallbackID = ""
	return cartResponse{Cart: c, TotalCents: c.TotalCents()}
}

func upsertItem(items []domain.CartItem, p domain.Product, qty int) []domain.CartItem {
	for i, item := range items {
		if item.ProductID == p.ID {
			items[i].Quantity += qty
			return items
		}
	}
	return append(items, domain.CartItem{
		ProductID: p.ID,
		Name:      p.Name,
		Emoji:     p.Emoji,
		UnitCents: p.PriceCents,
		Quantity:  qty,
	})
}

func setQty(items []domain.CartItem, productID string, qty int) []domain.CartItem {
	for i, item := range items {
		if item.ProductID == productID {
			items[i].Quantity = qty
			return items
		}
	}
	return items
}

func removeItem(items []domain.CartItem, productID string) []domain.CartItem {
	out := items[:0]
	for _, item := range items {
		if item.ProductID != productID {
			out = append(out, item)
		}
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, errors.New("não encontrado"))
	case errors.Is(err, store.ErrInsufficientStock):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, store.ErrCartClosed), errors.Is(err, store.ErrConflict):
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, store.ErrSessionExpired):
		writeError(w, http.StatusUnauthorized, err)
	default:
		writeError(w, http.StatusInternalServerError, err)
	}
}
