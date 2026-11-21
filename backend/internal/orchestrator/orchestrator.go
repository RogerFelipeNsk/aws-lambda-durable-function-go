package orchestrator

import (
	"context"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/domain"
)

type Orchestrator interface {
	StartCartSession(ctx context.Context, cartID string) (executionARN string, err error)
	SignalCheckout(ctx context.Context, callbackID string, signal domain.CheckoutSignal) error
	SignalPixPaid(ctx context.Context, callbackID string) error
	SignalFailure(ctx context.Context, callbackID, errorType, message string) error
}
