package store

import (
	"context"
	"errors"
	"time"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/domain"
)

var (
	ErrNotFound          = errors.New("not found")
	ErrConflict          = errors.New("conflict")
	ErrInsufficientStock = errors.New("estoque insuficiente")
	ErrCartClosed        = errors.New("carrinho não aceita alterações")
	ErrSessionExpired    = errors.New("sessão expirada")
)

type Store interface {
	PutProduct(ctx context.Context, p domain.Product) error
	GetProduct(ctx context.Context, id string) (domain.Product, error)
	ListProducts(ctx context.Context) ([]domain.Product, error)

	CreateUser(ctx context.Context, u domain.User) error
	GetUser(ctx context.Context, id string) (domain.User, error)

	CreateSession(ctx context.Context, s domain.Session) error
	GetSession(ctx context.Context, id string) (domain.Session, error)

	PutCart(ctx context.Context, c domain.Cart) error
	GetCart(ctx context.Context, id string) (domain.Cart, error)
	GetOpenCartByUser(ctx context.Context, userID string) (domain.Cart, error)

	PutOrder(ctx context.Context, o domain.Order) error
	GetOrder(ctx context.Context, id string) (domain.Order, error)
	ListOrdersByUser(ctx context.Context, userID string) ([]domain.Order, error)

	Reserve(ctx context.Context, cartID, productID string, qty int) (domain.Product, error)
	ReleaseCart(ctx context.Context, cartID string) error
	CommitCart(ctx context.Context, cartID string) error
}

func AppendCartEvent(c *domain.Cart, typ, msg string) {
	c.Events = domain.AppendEvent(c.Events, typ, msg)
	c.UpdatedAt = time.Now().UTC()
}

func AppendOrderEvent(o *domain.Order, typ, msg string) {
	o.Events = domain.AppendEvent(o.Events, typ, msg)
	o.UpdatedAt = time.Now().UTC()
}
