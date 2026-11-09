package mcpx

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Meter is called once per tool invocation before the tool body runs. It is the
// product's hook for quota enforcement and request accounting; returning a non-nil
// error aborts the call (the error is returned to the agent as-is).
type Meter interface {
	Meter(ctx context.Context, id Identity, tool string) error
}

// ToolServer binds an mcp.Server to the request Identity plus the product's
// Meter and ErrorMapper. ReadTool/WriteTool register handlers against it so the
// metering and error-mapping policy is uniform across every tool.
type ToolServer struct {
	MCP      *mcp.Server
	Identity Identity
	Meter    Meter
	Mapper   ErrorMapper
}

// ReadTool registers a metered, read-only tool. The handler meters the call,
// runs fn, and maps any error through ts.Mapper before returning it to the agent.
func ReadTool[IN any](ts *ToolServer, name, desc string, fn func(context.Context, IN) (map[string]any, error)) {
	mcp.AddTool(ts.MCP, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, _ *mcp.CallToolRequest, in IN) (*mcp.CallToolResult, map[string]any, error) {
			if err := ts.Meter.Meter(ctx, ts.Identity, name); err != nil {
				return nil, nil, err
			}
			out, err := fn(ctx, in)
			if err != nil {
				return nil, nil, ts.Mapper.MapError(err, name)
			}
			return nil, out, nil
		})
}

// WriteTool registers a metered, write-scoped tool. A read-scoped identity is
// refused with Coded("write_scope_required") BEFORE any work or metering happens,
// so a read key never touches product state or consumes quota on a write tool.
func WriteTool[IN any](ts *ToolServer, name, desc string, fn func(context.Context, IN) (map[string]any, error)) {
	mcp.AddTool(ts.MCP, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, _ *mcp.CallToolRequest, in IN) (*mcp.CallToolResult, map[string]any, error) {
			if !ts.Identity.CanWrite() {
				return nil, nil, Coded("write_scope_required")
			}
			if err := ts.Meter.Meter(ctx, ts.Identity, name); err != nil {
				return nil, nil, err
			}
			out, err := fn(ctx, in)
			if err != nil {
				return nil, nil, ts.Mapper.MapError(err, name)
			}
			return nil, out, nil
		})
}
