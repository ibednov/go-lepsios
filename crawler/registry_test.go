package crawler

import (
	"context"
	"encoding/json"
	"testing"

)

type registryTestAdapter struct{ code string }

func (a registryTestAdapter) Code() string                                          { return a.code }
func (a registryTestAdapter) Discover(context.Context) ([]ProductRef, error) { return nil, nil }
func (a registryTestAdapter) Fetch(context.Context, ProductRef) ([]RawOffer, error) {
	return nil, nil
}

func TestForSourceRejectsUnknownConfiguredAdapterInsteadOfFallingBack(t *testing.T) {
	legacy := registryTestAdapter{code: "shop"}
	r := NewRegistry(legacy)
	r.RegisterFactory("jsonld", func(SourceConfig) (Adapter, error) { return registryTestAdapter{code: "shop"}, nil })
	got, err := r.ForSource(SourceConfig{Code: "shop", Adapter: "typo", Enabled: true, AdapterConfig: json.RawMessage(`{}`)})
	if err == nil || got != nil {
		t.Fatalf("unknown configured adapter fell back: got=%v err=%v", got, err)
	}
	got, err = r.ForSource(SourceConfig{Code: "shop", Adapter: "shop", Enabled: true})
	if err != nil || got != legacy {
		t.Fatalf("legacy adapter resolution got=%v err=%v", got, err)
	}
}
