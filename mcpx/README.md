# mcpx

Product-neutral building blocks for an MCP server over Streamable HTTP: key
secrets (`IssueSecret`/`HashSecret`/`VerifySecret`), read/write scopes, a
context-bound `Identity`, coded errors with a sentinel→code `TableMapper`, ISO
date-range + pagination helpers, and an auth `Middleware` + stateless
`NewHandler` with metered `ReadTool`/`WriteTool` registration. It carries no
product types, SQL, or business defaults — a product supplies those via adapters.

```go
mw := mcpx.Middleware{
	KeyIDHeader:      "X-App-Key-ID",
	KeyHeader:        "X-App-Key",
	Authenticator:    appAuthenticator{deps},        // resolves mcpx.Identity
	UnauthorizedCode: "key_unauthorized",
	AuthErrorCode:    "key_authentication_failed",
	PreAuth:          []mcpx.Check{featureFlagCheck},  // e.g. global flag -> 403
	PostAuth:         []mcpx.IdentityCheck{quotaCheck}, // e.g. per-subject quota -> 429
	OnError:          func(err error) { log.Error().Err(err).Msg("mcp auth") },
}

cfg := mcpx.HandlerConfig{
	MaxBodyBytes: 2 << 20,
	NewServer: func(ctx context.Context, id mcpx.Identity) *mcp.Server {
		server := mcp.NewServer(&mcp.Implementation{Name: "app", Version: "1.0.0"}, nil)
		ts := &mcpx.ToolServer{MCP: server, Identity: id, Meter: appMeter{deps}, Mapper: appMapper}

		mcpx.ReadTool(ts, "list_items", "List items", func(ctx context.Context, in listInput) (map[string]any, error) {
			return deps.List(ctx, in)
		})
		// Write tools refuse read-scoped keys before any work/metering.
		mcpx.WriteTool(ts, "create_item", "Create an item", func(ctx context.Context, in createInput) (map[string]any, error) {
			if !id.CanWrite() { /* already guarded by WriteTool */ }
			return deps.Create(ctx, in) // id.Scope == mcpx.ScopeWrite reaches here
		})
		return server
	},
}

handler := mcpx.NewHandler(cfg, mw)
// The product may further wrap `handler` with its own process-local RPS limiter.
```

## TODO / future

A product-neutral process-local RPS limiter and a shared SQL-backed credential
store could later move here. For now products keep their own rate limiter
(composed OUTSIDE `NewHandler`) and their own SQL key storage behind an
`Authenticator` adapter.
