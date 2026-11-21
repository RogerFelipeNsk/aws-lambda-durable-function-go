package store

import (
	"testing"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/domain"
)

func TestReserveReleaseCommit(t *testing.T) {
	ctx := t.Context()
	m := NewMemory()
	p := domain.Product{ID: "bola", Available: 10, Reservations: map[string]int{}}
	if err := m.PutProduct(ctx, p); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Reserve(ctx, "cart1", "bola", 3); err != nil {
		t.Fatal(err)
	}
	got, _ := m.GetProduct(ctx, "bola")
	if got.Available != 7 || got.Reserved != 3 {
		t.Fatalf("after reserve: %+v", got)
	}
	if _, err := m.Reserve(ctx, "cart1", "bola", 8); err != ErrInsufficientStock {
		t.Fatalf("want insufficient, got %v", err)
	}
	if err := m.ReleaseCart(ctx, "cart1"); err != nil {
		t.Fatal(err)
	}
	got, _ = m.GetProduct(ctx, "bola")
	if got.Available != 10 || got.Reserved != 0 || got.Reservations["cart1"] != 0 {
		t.Fatalf("after release: %+v", got)
	}
	if _, err := m.Reserve(ctx, "cart1", "bola", 2); err != nil {
		t.Fatal(err)
	}
	if err := m.CommitCart(ctx, "cart1"); err != nil {
		t.Fatal(err)
	}
	got, _ = m.GetProduct(ctx, "bola")
	if got.Available != 8 || got.Reserved != 0 || got.Sold != 2 {
		t.Fatalf("after commit: %+v", got)
	}
}

func TestReleaseIsIdempotent(t *testing.T) {
	ctx := t.Context()
	m := NewMemory()
	_ = m.PutProduct(ctx, domain.Product{ID: "bola", Available: 5, Reservations: map[string]int{}})
	_, _ = m.Reserve(ctx, "c", "bola", 1)
	_ = m.ReleaseCart(ctx, "c")
	_ = m.ReleaseCart(ctx, "c")
	got, _ := m.GetProduct(ctx, "bola")
	if got.Available != 5 || got.Reserved != 0 {
		t.Fatalf("%+v", got)
	}
}
