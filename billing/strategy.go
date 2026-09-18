package billing

import (
	"context"
	"errors"
)

var (
	ErrNilIntent          = errors.New("billing: nil payment intent")
	ErrInvalidIntentState = errors.New("billing: invalid payment intent status for strategy")
	ErrUnknownStrategy    = errors.New("billing: unknown payment strategy")
)

// Strategy confirms or rejects a payment intent.
// Real gateways (Stripe, YooKassa, …) implement the same contract later.
type Strategy interface {
	Name() string
	Confirm(ctx context.Context, intent *PaymentIntent) error
}

// Registry maps strategy names to implementations.
type Registry struct {
	byName map[string]Strategy
}

// NewRegistry returns an empty strategy registry.
func NewRegistry() *Registry {
	return &Registry{byName: make(map[string]Strategy)}
}

// Register adds or replaces a strategy by Name().
func (r *Registry) Register(s Strategy) {
	if r == nil || s == nil {
		return
	}
	r.byName[s.Name()] = s
}

// Get returns a strategy by name.
func (r *Registry) Get(name string) (Strategy, error) {
	if r == nil {
		return nil, ErrUnknownStrategy
	}
	s, ok := r.byName[name]
	if !ok || s == nil {
		return nil, ErrUnknownStrategy
	}
	return s, nil
}
