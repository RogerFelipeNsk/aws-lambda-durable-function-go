package orchestrator

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-durable-execution-sdk-go/durable"
	"github.com/aws/aws-durable-execution-sdk-go/durable/durabletest"

	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/config"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/domain"
	"github.com/rogerpereira/aws-lambda-durable-function-go/internal/store"
)

type session struct {
	mu     sync.Mutex
	runner *durabletest.LocalRunner[domain.OrchestratorEvent, domain.OrchestratorResult]
	event  domain.OrchestratorEvent
	stop   chan struct{}
}

type Local struct {
	store  store.Store
	cfg    config.Config
	handle durable.Handler[domain.OrchestratorEvent, domain.OrchestratorResult]

	mu       sync.Mutex
	sessions map[string]*session
}

func NewLocal(st store.Store, cfg config.Config, handle durable.Handler[domain.OrchestratorEvent, domain.OrchestratorResult]) *Local {
	return &Local{
		store:    st,
		cfg:      cfg,
		handle:   handle,
		sessions: make(map[string]*session),
	}
}

func (l *Local) StartCartSession(_ context.Context, cartID string) (string, error) {
	l.mu.Lock()
	if _, exists := l.sessions[cartID]; exists {
		l.mu.Unlock()
		return "local:" + cartID, nil
	}
	s := &session{
		runner: durabletest.NewLocalRunner(l.handle),
		event:  domain.OrchestratorEvent{CartID: cartID},
		stop:   make(chan struct{}),
	}
	l.sessions[cartID] = s
	l.mu.Unlock()

	s.mu.Lock()
	_, err := s.runner.RunUntilComplete(s.event)
	s.mu.Unlock()
	if err != nil {
		return "", err
	}

	go l.watchCheckout(s, cartID)
	return "local:" + cartID, nil
}

func (l *Local) SignalCheckout(_ context.Context, callbackID string, signal domain.CheckoutSignal) error {
	s, err := l.byCallback(callbackID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.runner.SendCallbackSuccess(callbackID, signal); err != nil {
		return err
	}
	if _, err := s.runner.RunUntilComplete(s.event); err != nil {
		return err
	}
	go l.watchPix(s)
	return nil
}

func (l *Local) SignalPixPaid(_ context.Context, callbackID string) error {
	s, err := l.byCallback(callbackID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.runner.SendCallbackSuccess(callbackID, domain.PixSignal{Paid: true}); err != nil {
		return err
	}
	_, err = s.runner.RunUntilComplete(s.event)
	return err
}

func (l *Local) SignalFailure(_ context.Context, callbackID, errorType, message string) error {
	s, err := l.byCallback(callbackID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.runner.SendCallbackFailure(callbackID, errorType, message); err != nil {
		return err
	}
	_, err = s.runner.RunUntilComplete(s.event)
	return err
}

func (l *Local) watchCheckout(s *session, cartID string) {
	timer := time.NewTimer(l.cfg.CartTTL)
	defer timer.Stop()
	select {
	case <-s.stop:
		return
	case <-timer.C:
	}
	cart, err := l.store.GetCart(context.Background(), cartID)
	if err != nil || cart.Status != domain.CartOpen || cart.CheckoutCallbackID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.runner.TimeoutCallback(cart.CheckoutCallbackID)
	_, _ = s.runner.RunUntilComplete(s.event)
}

func (l *Local) watchPix(s *session) {
	timer := time.NewTimer(l.cfg.PixTTL)
	defer timer.Stop()
	select {
	case <-s.stop:
		return
	case <-timer.C:
	}
	cart, err := l.store.GetCart(context.Background(), s.event.CartID)
	if err != nil || cart.PixCallbackID == "" {
		return
	}
	if cart.Status != domain.CartCheckingOut {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.runner.TimeoutCallback(cart.PixCallbackID)
	_, _ = s.runner.RunUntilComplete(s.event)
}

func (l *Local) byCallback(callbackID string) (*session, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range l.sessions {
		s.mu.Lock()
		for _, cb := range s.runner.OpenCallbacks() {
			if cb.CallbackID == callbackID {
				s.mu.Unlock()
				return s, nil
			}
		}
		s.mu.Unlock()
	}
	return nil, fmt.Errorf("callback not found")
}
