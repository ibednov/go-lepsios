# httpx

Gin HTTP engine, middleware, standardized API responses, and health checks.

```go
import "github.com/ibednov/go-lepsios/httpx"

engine := httpx.ProvideEngine(cfg, bundle, logger)
httpx.RegisterHealth(engine, liveness, readiness)
```

## Response envelopes

Two JSON shapes live under `httpx/response`:

| Package | Shape | Use |
|---|---|---|
| `response` | `{ data, meta?, message? }` | newer / wishimi-style APIs |
| `response/envelope` | `{ status, data, errors, meta }` | Laravel / md-tests style |

Shared pagination helpers (`Pagination`, `NormalizePage`, `NewPagination`, `Offset`) are in `response` and re-exported from `envelope`.
