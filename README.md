# go-lepsios

Multi-module Go infrastructure library for Alepsios backend services.

## Modules (Phase 1)

| Module | Import path | Description |
|--------|-------------|-------------|
| `log` | `github.com/ibednov/go-lepsios/log` | Structured logging (`zerolog`) |
| `i18n` | `github.com/ibednov/go-lepsios/i18n` | Localization bundle |
| `httpx` | `github.com/ibednov/go-lepsios/httpx` | Gin engine, middleware, API responses |


## Local development

```bash
make tools   # once: installs golangci-lint (needs $GOPATH/bin in PATH)
make tidy
make test
make lint
```

### Consumer `replace` (dev)

```go
// eco-back/go.mod
replace github.com/ibednov/go-lepsios/log => ../go-lepsios/log
```

Or add both repos to a shared `go.work`.

## Versioning

Semver tags per module: `log/v0.1.0`, `httpx/v0.1.0`, etc.
