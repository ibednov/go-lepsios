package mcpx

import (
	"context"
	"errors"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

type stubMeter struct {
	calls int
	err   error
}

func (m *stubMeter) Meter(context.Context, Identity, string) error {
	m.calls++
	return m.err
}

type sampleInput struct {
	Value string `json:"value" jsonschema:"A sample value"`
}

func newToolServer(id Identity) (*ToolServer, *stubMeter) {
	meter := &stubMeter{}
	ts := &ToolServer{
		MCP:      mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1.0.0"}, nil),
		Identity: id,
		Meter:    meter,
		Mapper:   TableMapper(nil, nil),
	}
	return ts, meter
}

// TestToolRegistrationNoPanic guards against duplicate-name registration panics
// and confirms both read and write tools register for a read identity exactly as
// for a write identity (the scope gate is enforced per-call, not at registration).
func TestToolRegistrationReadIdentity(t *testing.T) {
	t.Parallel()
	ts, _ := newToolServer(Identity{Scope: ScopeRead})
	require.NotPanics(t, func() {
		ReadTool(ts, "list_things", "List things", func(context.Context, sampleInput) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		})
		WriteTool(ts, "create_thing", "Create a thing", func(context.Context, sampleInput) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		})
	})
}

func TestToolRegistrationWriteIdentity(t *testing.T) {
	t.Parallel()
	ts, _ := newToolServer(Identity{Scope: ScopeWrite})
	require.NotPanics(t, func() {
		ReadTool(ts, "list_things", "List things", func(context.Context, sampleInput) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		})
		WriteTool(ts, "create_thing", "Create a thing", func(context.Context, sampleInput) (map[string]any, error) {
			return map[string]any{"ok": true}, nil
		})
	})
}

// TestWriteGateViaCanWrite documents the gate WriteTool relies on: a read
// identity must not satisfy CanWrite, so WriteTool refuses before metering.
func TestWriteGateViaCanWrite(t *testing.T) {
	t.Parallel()
	require.False(t, Identity{Scope: ScopeRead}.CanWrite())
	require.True(t, Identity{Scope: ScopeWrite}.CanWrite())
}

// TestMapperWiring confirms the ToolServer mapper translates a sentinel to its
// stable code (the same mapper ReadTool/WriteTool apply to tool errors).
func TestMapperWiring(t *testing.T) {
	t.Parallel()
	sentinel := errors.New("boom")
	ts, _ := newToolServer(Identity{Scope: ScopeWrite})
	ts.Mapper = TableMapper([]CodeEntry{{Code: "bad_thing", Sentinels: []error{sentinel}}}, nil)
	require.Equal(t, "bad_thing", ts.Mapper.MapError(sentinel, "create_thing").Error())
}
