package mailer

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

// Receipt identifies a message accepted by a provider; it does not prove delivery.
type Receipt struct {
	Provider  string
	MessageID string
}

// Strategy sends a message using one provider.
type Strategy interface {
	Deliver(context.Context, Message) (Receipt, error)
}

// Router selects a registered strategy by name for each send.
type Router struct {
	strategies map[string]Strategy
}

func NewRouter(strategies map[string]Strategy) (*Router, error) {
	if len(strategies) == 0 {
		return nil, errors.New("mailer: at least one strategy is required")
	}
	copyOf := make(map[string]Strategy, len(strategies))
	for name, strategy := range strategies {
		if name == "" || strategy == nil || (reflect.ValueOf(strategy).Kind() == reflect.Pointer && reflect.ValueOf(strategy).IsNil()) {
			return nil, errors.New("mailer: strategy name and implementation are required")
		}
		copyOf[name] = strategy
	}
	return &Router{strategies: copyOf}, nil
}

func (r *Router) Send(ctx context.Context, strategy string, message Message) (Receipt, error) {
	if r == nil {
		return Receipt{}, errors.New("mailer: router is nil")
	}
	sender, ok := r.strategies[strategy]
	if !ok {
		return Receipt{}, fmt.Errorf("mailer: unknown strategy %q", strategy)
	}
	return sender.Deliver(ctx, message)
}
