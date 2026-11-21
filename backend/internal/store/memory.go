package store

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/domain"
)

type Memory struct {
	mu       sync.Mutex
	products map[string]domain.Product
	users    map[string]domain.User
	sessions map[string]domain.Session
	carts    map[string]domain.Cart
	orders   map[string]domain.Order
}

func NewMemory() *Memory {
	return &Memory{
		products: make(map[string]domain.Product),
		users:    make(map[string]domain.User),
		sessions: make(map[string]domain.Session),
		carts:    make(map[string]domain.Cart),
		orders:   make(map[string]domain.Order),
	}
}

func (m *Memory) PutProduct(_ context.Context, p domain.Product) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if p.Reservations == nil {
		p.Reservations = make(map[string]int)
	} else {
		p.Reservations = maps.Clone(p.Reservations)
	}
	m.products[p.ID] = p
	return nil
}

func (m *Memory) GetProduct(_ context.Context, id string) (domain.Product, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.products[id]
	if !ok {
		return domain.Product{}, ErrNotFound
	}
	return cloneProduct(p), nil
}

func (m *Memory) ListProducts(_ context.Context) ([]domain.Product, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.Product, 0, len(m.products))
	for _, p := range m.products {
		out = append(out, cloneProduct(p))
	}
	slices.SortFunc(out, func(a, b domain.Product) int {
		if a.Category != b.Category {
			if a.Category < b.Category {
				return -1
			}
			return 1
		}
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	return out, nil
}

func (m *Memory) CreateUser(_ context.Context, u domain.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = u
	return nil
}

func (m *Memory) GetUser(_ context.Context, id string) (domain.User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return domain.User{}, ErrNotFound
	}
	return u, nil
}

func (m *Memory) CreateSession(_ context.Context, s domain.Session) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[s.ID] = s
	return nil
}

func (m *Memory) GetSession(_ context.Context, id string) (domain.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return domain.Session{}, ErrNotFound
	}
	if time.Now().UTC().After(s.ExpiresAt) {
		return domain.Session{}, ErrSessionExpired
	}
	return s, nil
}

func (m *Memory) PutCart(_ context.Context, c domain.Cart) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c.Items = slices.Clone(c.Items)
	c.Events = slices.Clone(c.Events)
	m.carts[c.ID] = c
	return nil
}

func (m *Memory) GetCart(_ context.Context, id string) (domain.Cart, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.carts[id]
	if !ok {
		return domain.Cart{}, ErrNotFound
	}
	return cloneCart(c), nil
}

func (m *Memory) GetOpenCartByUser(_ context.Context, userID string) (domain.Cart, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var found domain.Cart
	var ok bool
	for _, c := range m.carts {
		if c.UserID != userID || c.IsTerminal() {
			continue
		}
		if !ok || c.CreatedAt.After(found.CreatedAt) {
			found = c
			ok = true
		}
	}
	if !ok {
		return domain.Cart{}, ErrNotFound
	}
	return cloneCart(found), nil
}

func (m *Memory) PutOrder(_ context.Context, o domain.Order) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	o.Items = slices.Clone(o.Items)
	o.Events = slices.Clone(o.Events)
	if o.Pix != nil {
		cp := *o.Pix
		o.Pix = &cp
	}
	m.orders[o.ID] = o
	return nil
}

func (m *Memory) GetOrder(_ context.Context, id string) (domain.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[id]
	if !ok {
		return domain.Order{}, ErrNotFound
	}
	return cloneOrder(o), nil
}

func (m *Memory) ListOrdersByUser(_ context.Context, userID string) ([]domain.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]domain.Order, 0)
	for _, o := range m.orders {
		if o.UserID == userID {
			out = append(out, cloneOrder(o))
		}
	}
	slices.SortFunc(out, func(a, b domain.Order) int {
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	return out, nil
}

func (m *Memory) Reserve(_ context.Context, cartID, productID string, qty int) (domain.Product, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p, ok := m.products[productID]
	if !ok {
		return domain.Product{}, ErrNotFound
	}
	if p.Reservations == nil {
		p.Reservations = make(map[string]int)
	}
	if qty == 0 {
		return cloneProduct(p), nil
	}
	if qty > 0 && p.Available < qty {
		return domain.Product{}, ErrInsufficientStock
	}
	if qty < 0 {
		held := p.Reservations[cartID]
		if held < -qty {
			return domain.Product{}, ErrConflict
		}
	}
	p.Available -= qty
	p.Reserved += qty
	p.Reservations[cartID] += qty
	if p.Reservations[cartID] == 0 {
		delete(p.Reservations, cartID)
	}
	m.products[productID] = p
	return cloneProduct(p), nil
}

func (m *Memory) ReleaseCart(_ context.Context, cartID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.releaseLocked(cartID)
}

func (m *Memory) CommitCart(_ context.Context, cartID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, p := range m.products {
		qty := p.Reservations[cartID]
		if qty == 0 {
			continue
		}
		p.Reserved -= qty
		p.Sold += qty
		delete(p.Reservations, cartID)
		m.products[id] = p
	}
	return nil
}

func (m *Memory) releaseLocked(cartID string) error {
	for id, p := range m.products {
		qty := p.Reservations[cartID]
		if qty == 0 {
			continue
		}
		p.Available += qty
		p.Reserved -= qty
		delete(p.Reservations, cartID)
		m.products[id] = p
	}
	return nil
}

func cloneProduct(p domain.Product) domain.Product {
	p.Reservations = maps.Clone(p.Reservations)
	if p.Reservations == nil {
		p.Reservations = make(map[string]int)
	}
	return p
}

func cloneCart(c domain.Cart) domain.Cart {
	c.Items = slices.Clone(c.Items)
	c.Events = slices.Clone(c.Events)
	if c.ExpiresAt != nil {
		t := *c.ExpiresAt
		c.ExpiresAt = &t
	}
	return c
}

func cloneOrder(o domain.Order) domain.Order {
	o.Items = slices.Clone(o.Items)
	o.Events = slices.Clone(o.Events)
	if o.Pix != nil {
		cp := *o.Pix
		o.Pix = &cp
	}
	return o
}
