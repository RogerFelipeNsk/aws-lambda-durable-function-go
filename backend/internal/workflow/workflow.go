package workflow

import (
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-durable-execution-sdk-go/durable"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/config"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/domain"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/payment"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/pix"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/store"
)

type Handler struct {
	store store.Store
	cfg   config.Config
}

func New(st store.Store, cfg config.Config) durable.Handler[domain.OrchestratorEvent, domain.OrchestratorResult] {
	h := Handler{store: st, cfg: cfg}
	return h.Run
}

func (h Handler) Run(ctx durable.Context, event domain.OrchestratorEvent) (domain.OrchestratorResult, error) {
	if event.CartID == "" {
		return domain.OrchestratorResult{}, errors.New("cartId is required")
	}

	signal, err := durable.WaitForCallback[domain.CheckoutSignal](ctx, "await-checkout",
		func(sc durable.StepContext, callbackID string) error {
			return h.onCheckoutReady(sc, event.CartID, callbackID)
		},
		durable.WithCallbackTimeout(h.cfg.CartTTL),
		durable.WithCallbackSerdes(durable.JSONSerdes),
		durable.WithSubmitterRetry(durable.NoRetry()),
	)
	if err != nil {
		if isCallbackGone(err) {
			return h.expireCart(ctx, event, "carrinho expirado — unidades reservadas voltaram ao estoque")
		}
		return domain.OrchestratorResult{}, err
	}

	if _, err := durable.Step(ctx, "mark-checkout", func(sc durable.StepContext) (string, error) {
		return "", h.markCheckingOut(sc, event.CartID, signal)
	}, durable.WithRetry(durable.NoRetry())); err != nil {
		return domain.OrchestratorResult{}, err
	}

	switch signal.Method {
	case domain.PayPix:
		return h.runPix(ctx, event, signal)
	case domain.PayCard:
		return h.runCard(ctx, event, signal)
	default:
		return h.failPayment(ctx, event, signal.OrderID, "método de pagamento inválido")
	}
}

func (h Handler) onCheckoutReady(sc durable.StepContext, cartID, callbackID string) error {
	cart, err := h.store.GetCart(sc, cartID)
	if err != nil {
		return err
	}
	cart.CheckoutCallbackID = callbackID
	cart.Status = domain.CartOpen
	exp := time.Now().UTC().Add(h.cfg.CartTTL)
	cart.ExpiresAt = &exp
	store.AppendCartEvent(&cart, "session_open", fmt.Sprintf("reserva ativa por %s", h.cfg.CartTTL))
	return h.store.PutCart(sc, cart)
}

func (h Handler) markCheckingOut(sc durable.StepContext, cartID string, signal domain.CheckoutSignal) error {
	cart, err := h.store.GetCart(sc, cartID)
	if err != nil {
		return err
	}
	cart.Status = domain.CartCheckingOut
	cart.OrderID = signal.OrderID
	store.AppendCartEvent(&cart, "checkout", "checkout iniciado via "+signal.Method)
	if err := h.store.PutCart(sc, cart); err != nil {
		return err
	}
	order, err := h.store.GetOrder(sc, signal.OrderID)
	if err != nil {
		return err
	}
	if signal.Method == domain.PayCard {
		order.Status = domain.OrderProcessingCard
		store.AppendOrderEvent(&order, "card_processing", "enviando cobrança à operadora fictícia")
	} else {
		order.Status = domain.OrderPendingPayment
		store.AppendOrderEvent(&order, "pix_prepare", "gerando cobrança PIX")
	}
	return h.store.PutOrder(sc, order)
}

func (h Handler) runPix(ctx durable.Context, event domain.OrchestratorEvent, signal domain.CheckoutSignal) (domain.OrchestratorResult, error) {
	_, err := durable.WaitForCallback[domain.PixSignal](ctx, "await-pix",
		func(sc durable.StepContext, callbackID string) error {
			return h.startPix(sc, event.CartID, signal.OrderID, callbackID)
		},
		durable.WithCallbackTimeout(h.cfg.PixTTL),
		durable.WithCallbackSerdes(durable.JSONSerdes),
		durable.WithSubmitterRetry(durable.NoRetry()),
	)
	if err != nil {
		if isCallbackGone(err) {
			return h.expirePix(ctx, event, signal.OrderID)
		}
		return domain.OrchestratorResult{}, err
	}
	return h.commitPaid(ctx, event, signal.OrderID, domain.PayPix, "")
}

func (h Handler) startPix(sc durable.StepContext, cartID, orderID, callbackID string) error {
	order, err := h.store.GetOrder(sc, orderID)
	if err != nil {
		return err
	}
	expires := time.Now().UTC().Add(h.cfg.PixTTL)
	charge := domain.PixCharge{
		TxID:       orderID,
		CopiaECola: pix.CopiaECola(orderID, "Arena Store Demo", "SAO PAULO", "arena@demo.local", order.TotalCents),
		ExpiresAt:  expires,
	}
	order.Pix = &charge
	order.Status = domain.OrderAwaitingPix
	store.AppendOrderEvent(&order, "pix_qr", fmt.Sprintf("QR PIX gerado, expira em %s", h.cfg.PixTTL))
	if err := h.store.PutOrder(sc, order); err != nil {
		return err
	}
	cart, err := h.store.GetCart(sc, cartID)
	if err != nil {
		return err
	}
	cart.PixCallbackID = callbackID
	store.AppendCartEvent(&cart, "pix_waiting", "aguardando pagamento PIX")
	return h.store.PutCart(sc, cart)
}

func (h Handler) runCard(ctx durable.Context, event domain.OrchestratorEvent, signal domain.CheckoutSignal) (domain.OrchestratorResult, error) {
	last4, err := durable.Step(ctx, "charge-card", func(sc durable.StepContext) (string, error) {
		delay := h.cfg.CardDelay
		if signal.Simulate == "card_delay" {
			delay = h.cfg.CardDelayLong
		}
		if delay > 0 {
			select {
			case <-sc.Done():
				return "", sc.Err()
			case <-time.After(delay):
			}
		}
		if signal.Simulate == "card_fail" {
			return "", payment.ErrDeclined
		}
		in := payment.Input{}
		if signal.Card != nil {
			in = payment.Input{Number: signal.Card.Number, Exp: signal.Card.Exp, CVC: signal.Card.CVC}
		}
		if err := payment.Validate(in); err != nil {
			return "", err
		}
		return payment.Last4(in.Number), nil
	}, durable.WithRetry(durable.NoRetry()))

	if err != nil {
		var stepErr *durable.StepError
		if errors.As(err, &stepErr) {
			return h.failPayment(ctx, event, signal.OrderID, stepErr.Message)
		}
		return domain.OrchestratorResult{}, err
	}
	return h.commitPaid(ctx, event, signal.OrderID, domain.PayCard, last4)
}

func (h Handler) commitPaid(ctx durable.Context, event domain.OrchestratorEvent, orderID, method, last4 string) (domain.OrchestratorResult, error) {
	if _, err := durable.Step(ctx, "commit-stock", func(sc durable.StepContext) (string, error) {
		if err := h.store.CommitCart(sc, event.CartID); err != nil {
			return "", err
		}
		cart, err := h.store.GetCart(sc, event.CartID)
		if err != nil {
			return "", err
		}
		cart.Status = domain.CartPaid
		cart.ExpiresAt = nil
		store.AppendCartEvent(&cart, "paid", "pagamento confirmado, estoque baixado")
		if err := h.store.PutCart(sc, cart); err != nil {
			return "", err
		}
		order, err := h.store.GetOrder(sc, orderID)
		if err != nil {
			return "", err
		}
		order.Status = domain.OrderPaid
		order.CardLast4 = last4
		msg := "pedido pago"
		if method == domain.PayPix {
			msg = "PIX confirmado"
		} else {
			msg = "cartão aprovado"
		}
		store.AppendOrderEvent(&order, "paid", msg)
		return "", h.store.PutOrder(sc, order)
	}, durable.WithRetry(durable.NoRetry())); err != nil {
		return domain.OrchestratorResult{}, err
	}
	return domain.OrchestratorResult{CartID: event.CartID, OrderID: orderID, Status: domain.OrderPaid}, nil
}

func (h Handler) expireCart(ctx durable.Context, event domain.OrchestratorEvent, message string) (domain.OrchestratorResult, error) {
	if _, err := durable.Step(ctx, "expire-cart", func(sc durable.StepContext) (string, error) {
		if err := h.store.ReleaseCart(sc, event.CartID); err != nil {
			return "", err
		}
		cart, err := h.store.GetCart(sc, event.CartID)
		if err != nil {
			return "", err
		}
		cart.Status = domain.CartExpired
		store.AppendCartEvent(&cart, "expired", message)
		if err := h.store.PutCart(sc, cart); err != nil {
			return "", err
		}
		if cart.OrderID == "" {
			return "", nil
		}
		order, err := h.store.GetOrder(sc, cart.OrderID)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return "", nil
			}
			return "", err
		}
		order.Status = domain.OrderCancelled
		store.AppendOrderEvent(&order, "cancelled", message)
		return "", h.store.PutOrder(sc, order)
	}, durable.WithRetry(durable.NoRetry())); err != nil {
		return domain.OrchestratorResult{}, err
	}
	return domain.OrchestratorResult{CartID: event.CartID, Status: domain.CartExpired}, nil
}

func (h Handler) expirePix(ctx durable.Context, event domain.OrchestratorEvent, orderID string) (domain.OrchestratorResult, error) {
	if _, err := durable.Step(ctx, "expire-pix", func(sc durable.StepContext) (string, error) {
		if err := h.store.ReleaseCart(sc, event.CartID); err != nil {
			return "", err
		}
		cart, err := h.store.GetCart(sc, event.CartID)
		if err != nil {
			return "", err
		}
		cart.Status = domain.CartPaymentFailed
		store.AppendCartEvent(&cart, "pix_expired", "PIX expirado — estoque devolvido")
		if err := h.store.PutCart(sc, cart); err != nil {
			return "", err
		}
		order, err := h.store.GetOrder(sc, orderID)
		if err != nil {
			return "", err
		}
		order.Status = domain.OrderPixExpired
		store.AppendOrderEvent(&order, "pix_expired", "cobrança PIX expirou sem pagamento")
		return "", h.store.PutOrder(sc, order)
	}, durable.WithRetry(durable.NoRetry())); err != nil {
		return domain.OrchestratorResult{}, err
	}
	return domain.OrchestratorResult{CartID: event.CartID, OrderID: orderID, Status: domain.OrderPixExpired}, nil
}

func (h Handler) failPayment(ctx durable.Context, event domain.OrchestratorEvent, orderID, message string) (domain.OrchestratorResult, error) {
	if _, err := durable.Step(ctx, "fail-payment", func(sc durable.StepContext) (string, error) {
		if err := h.store.ReleaseCart(sc, event.CartID); err != nil {
			return "", err
		}
		cart, err := h.store.GetCart(sc, event.CartID)
		if err != nil {
			return "", err
		}
		cart.Status = domain.CartPaymentFailed
		store.AppendCartEvent(&cart, "payment_failed", message)
		if err := h.store.PutCart(sc, cart); err != nil {
			return "", err
		}
		if orderID == "" {
			return "", nil
		}
		order, err := h.store.GetOrder(sc, orderID)
		if err != nil {
			return "", err
		}
		order.Status = domain.OrderDeclined
		store.AppendOrderEvent(&order, "declined", message)
		return "", h.store.PutOrder(sc, order)
	}, durable.WithRetry(durable.NoRetry())); err != nil {
		return domain.OrchestratorResult{}, err
	}
	return domain.OrchestratorResult{CartID: event.CartID, OrderID: orderID, Status: domain.OrderDeclined}, nil
}

func isCallbackGone(err error) bool {
	var timeout *durable.CallbackTimeoutError
	var external *durable.CallbackExternalError
	return errors.As(err, &timeout) || errors.As(err, &external)
}
