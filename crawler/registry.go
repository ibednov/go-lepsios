package crawler

import (
	"encoding/json"
	"fmt"
	"sync"
)

// Registry constructs a universal adapter from a source's persisted adapter type.
type Registry struct {
	mu        sync.RWMutex
	adapters  map[string]Adapter
	factories map[string]Factory
}

// SourceConfig is passed to factories for adapters configured per source row.
type SourceConfig struct {
	Code          string
	BaseURL       string
	Adapter       string
	Enabled       bool
	RateLimitRPS  float64
	AdapterConfig json.RawMessage
}

// Factory constructs an adapter for one database-configured source.
type Factory func(SourceConfig) (Adapter, error)

func NewRegistry(adapters ...Adapter) *Registry {
	r := &Registry{adapters: make(map[string]Adapter, len(adapters)), factories: make(map[string]Factory)}
	for _, a := range adapters {
		if a == nil {
			continue
		}
		r.adapters[a.Code()] = a
	}
	return r
}

func (r *Registry) RegisterFactory(kind string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if kind != "" && factory != nil {
		r.factories[kind] = factory
	}
}

// ForSource constructs the adapter selected by the persisted platform type.
func (r *Registry) ForSource(source SourceConfig) (Adapter, error) {
	r.mu.RLock()
	legacy, hasLegacy := r.adapters[source.Code]
	factory := r.factories[source.Adapter]
	r.mu.RUnlock()
	if !source.Enabled {
		return nil, fmt.Errorf("source %q is disabled", source.Code)
	}
	if hasLegacy && (source.Adapter == "" || source.Adapter == source.Code || source.Adapter == legacy.Code()) {
		return legacy, nil
	}
	if factory == nil {
		return nil, fmt.Errorf("%w: adapter %q for source %q", errUnsupported, source.Adapter, source.Code)
	}
	return factory(source)
}

func (r *Registry) Get(code string) (Adapter, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.adapters[code]
	if !ok {
		return nil, fmt.Errorf("%w: %s", errUnsupported, code)
	}
	return a, nil
}

func (r *Registry) Codes() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]string, 0, len(r.adapters))
	for c := range r.adapters {
		out = append(out, c)
	}
	return out
}

var errUnsupported = fmt.Errorf("unsupported source")
