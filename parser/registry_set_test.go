package parser

import (
	"context"
	"testing"
)

type stubStrategy struct {
	host string
	name string
}

func (s stubStrategy) CanHandle(host string) bool { return host == s.host }
func (s stubStrategy) Parse(context.Context, string) (RawParseResult, error) {
	return RawParseResult{Title: s.name}, nil
}
func (s stubStrategy) Name() string { return s.name }

func TestRegistrySetStrategies(t *testing.T) {
	r := NewRegistry(stubStrategy{host: "a.test", name: "a"})
	if got := r.StrategyNames(); len(got) != 1 || got[0] != "a" {
		t.Fatalf("names=%v", got)
	}
	if r.StrategyForHost("a.test") != "a" {
		t.Fatal("host a")
	}

	r.SetStrategies(stubStrategy{host: "b.test", name: "b"})
	if got := r.StrategyNames(); len(got) != 1 || got[0] != "b" {
		t.Fatalf("after set names=%v", got)
	}
	if r.StrategyForHost("a.test") != "unknown" {
		t.Fatal("a should be gone")
	}
	if r.StrategyForHost("b.test") != "b" {
		t.Fatal("host b")
	}
	res, err := r.Parse(context.Background(), "https://b.test/x")
	if err != nil || res.Title != "b" {
		t.Fatalf("parse: %v %+v", err, res)
	}
}
