package domain

import (
	"time"
)

const (
	CartStarting      = "starting"
	CartOpen          = "open"
	CartCheckingOut   = "checking_out"
	CartExpired       = "expired"
	CartPaid          = "paid"
	CartPaymentFailed = "payment_failed"
	CartCancelled     = "cancelled"

	OrderPendingPayment = "pending_payment"
	OrderAwaitingPix    = "awaiting_pix"
	OrderProcessingCard = "processing_card"
	OrderPaid           = "paid"
	OrderPixExpired     = "pix_expired"
	OrderDeclined       = "declined"
	OrderCancelled      = "cancelled"

	PayPix  = "pix"
	PayCard = "card"

	CurrencyBRL = "BRL"
)

type Product struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	Category     string         `json:"category"`
	PriceCents   int64          `json:"priceCents"`
	Currency     string         `json:"currency"`
	Emoji        string         `json:"emoji"`
	Available    int            `json:"available"`
	Reserved     int            `json:"reserved"`
	Sold         int            `json:"sold"`
	Reservations map[string]int `json:"reservations,omitempty"`
}

func (p Product) Public() Product {
	p.Reservations = nil
	return p
}

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"createdAt"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type CartItem struct {
	ProductID string `json:"productId"`
	Name      string `json:"name"`
	Emoji     string `json:"emoji"`
	UnitCents int64  `json:"unitCents"`
	Quantity  int    `json:"quantity"`
}

type TimelineEvent struct {
	At      time.Time `json:"at"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
}

type Cart struct {
	ID                 string          `json:"id"`
	UserID             string          `json:"userId"`
	Status             string          `json:"status"`
	Items              []CartItem      `json:"items"`
	ExecutionARN       string          `json:"executionArn,omitempty"`
	CheckoutCallbackID string          `json:"checkoutCallbackId,omitempty"`
	PixCallbackID      string          `json:"pixCallbackId,omitempty"`
	OrderID            string          `json:"orderId,omitempty"`
	ExpiresAt          *time.Time      `json:"expiresAt,omitempty"`
	Events             []TimelineEvent `json:"events"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
}

func (c Cart) TotalCents() int64 {
	var total int64
	for _, item := range c.Items {
		total += item.UnitCents * int64(item.Quantity)
	}
	return total
}

func (c Cart) IsTerminal() bool {
	switch c.Status {
	case CartExpired, CartPaid, CartPaymentFailed, CartCancelled:
		return true
	default:
		return false
	}
}

func (c Cart) ItemQty(productID string) int {
	for _, item := range c.Items {
		if item.ProductID == productID {
			return item.Quantity
		}
	}
	return 0
}

type PixCharge struct {
	TxID       string    `json:"txId"`
	CopiaECola string    `json:"copiaECola"`
	ExpiresAt  time.Time `json:"expiresAt"`
}

type Order struct {
	ID         string          `json:"id"`
	UserID     string          `json:"userId"`
	CartID     string          `json:"cartId"`
	Status     string          `json:"status"`
	Method     string          `json:"method"`
	Items      []CartItem      `json:"items"`
	TotalCents int64           `json:"totalCents"`
	Currency   string          `json:"currency"`
	Pix        *PixCharge      `json:"pix,omitempty"`
	CardLast4  string          `json:"cardLast4,omitempty"`
	Simulate   string          `json:"simulate,omitempty"`
	Events     []TimelineEvent `json:"events"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

type CheckoutSignal struct {
	OrderID  string    `json:"orderId"`
	Method   string    `json:"method"`
	Card     *CardData `json:"card,omitempty"`
	Simulate string    `json:"simulate,omitempty"`
}

type CardData struct {
	Number string `json:"number"`
	Exp    string `json:"exp"`
	CVC    string `json:"cvc"`
}

type PixSignal struct {
	Paid bool `json:"paid"`
}

type OrchestratorEvent struct {
	CartID string `json:"cartId"`
}

type OrchestratorResult struct {
	CartID  string `json:"cartId"`
	OrderID string `json:"orderId,omitempty"`
	Status  string `json:"status"`
}

func AppendEvent(events []TimelineEvent, typ, message string) []TimelineEvent {
	return append(events, TimelineEvent{
		At:      time.Now().UTC(),
		Type:    typ,
		Message: message,
	})
}
