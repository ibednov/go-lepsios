# httpx

Gin HTTP engine, middleware, standardized API responses, and health checks.

```go
import "github.com/ibednov/go-lepsios/httpx"

engine := httpx.ProvideEngine(cfg, bundle, logger)
httpx.RegisterHealth(engine, liveness, readiness)
```
