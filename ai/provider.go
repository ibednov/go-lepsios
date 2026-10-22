// Package ai contains provider-neutral contracts for domain-owned AI sidecars.
package ai

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrUnknownStrategy = errors.New("ai: unknown provider strategy")
var ErrInvalidStrategy = errors.New("ai: invalid provider strategy")
var ErrMissingCredential = errors.New("ai: missing provider credential")

// Credentials carries provider-specific secrets without prescribing persistence.
type Credentials map[string]string

// Turn is one user or assistant message in the conversation history.
type Turn struct {
	Role    string
	Content string
}

// Request is the provider-neutral subset of a domain's AI request.
type Request struct {
	Model           string
	ReasoningEffort string
	SystemPrompt    string
	History         []Turn
	Message         string
	MaxTokens       int
	StructuredJSON  bool
}

// Result contains generated text and provider usage diagnostics.
type Result struct {
	Text               string
	InputTokens        int
	OutputTokens       int
	ResponseID         string
	ResponseModel      string
	FinishReason       string
	NativeFinishReason string
	ReasoningLength    int
	ReasoningTokens    int
}

// Model describes a selectable provider model.
type Model struct {
	ID   string
	Name string
}

// Provider executes requests using one concrete provider configuration.
type Provider interface {
	Name() string
	ListModels(context.Context) ([]Model, error)
	Chat(context.Context, Request) (Result, error)
}

// Strategy validates credentials and creates a provider client for one vendor.
type Strategy interface {
	Name() string
	New(Credentials) (Provider, error)
}

// ProviderBuilder creates a provider client after a strategy validates its credentials.
// Host applications use builders to supply runtime configuration such as HTTP clients and endpoints.
type ProviderBuilder func(Credentials) (Provider, error)

type credentialStrategy struct {
	name     string
	required []string
	build    ProviderBuilder
}

func (s credentialStrategy) Name() string { return s.name }

func (s credentialStrategy) New(credentials Credentials) (Provider, error) {
	for _, key := range s.required {
		if strings.TrimSpace(credentials[key]) == "" {
			return nil, fmt.Errorf("%w: %s", ErrMissingCredential, key)
		}
	}
	if s.build == nil {
		return nil, ErrInvalidStrategy
	}
	return s.build(credentials)
}

// Registry selects a provider-specific strategy by its stable name.
type Registry struct {
	strategies map[string]Strategy
}

func NewRegistry(strategies ...Strategy) (*Registry, error) {
	r := &Registry{strategies: make(map[string]Strategy, len(strategies))}
	for _, strategy := range strategies {
		if strategy == nil || strategy.Name() == "" {
			return nil, ErrInvalidStrategy
		}
		if _, exists := r.strategies[strategy.Name()]; exists {
			return nil, fmt.Errorf("%w: duplicate name %q", ErrInvalidStrategy, strategy.Name())
		}
		r.strategies[strategy.Name()] = strategy
	}
	return r, nil
}

func (r *Registry) New(name string, credentials Credentials) (Provider, error) {
	if r == nil {
		return nil, ErrUnknownStrategy
	}
	strategy, ok := r.strategies[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownStrategy, name)
	}
	return strategy.New(credentials)
}
