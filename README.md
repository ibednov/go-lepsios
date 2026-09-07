# go-lepsios

Multi-module Go infrastructure library for Alepsios backend services.

## Modules

| Module | Import path | Description |
|--------|-------------|-------------|
| `log` | `github.com/ibednov/go-lepsios/log` | Structured logging (`zerolog`) |
| `i18n` | `github.com/ibednov/go-lepsios/i18n` | Localization bundle |
| `httpx` | `github.com/ibednov/go-lepsios/httpx` | Gin engine, middleware, API responses |
| `currency` | `github.com/ibednov/go-lepsios/currency` | ISO codes, official rates, BYN-hub converter |
| `exchange` | `github.com/ibednov/go-lepsios/exchange` | Country → FX provider registry (NBRB for BY) |
| `files` | `github.com/ibednov/go-lepsios/files` | File storage adapter (local / S3-MinIO) |
| `crypto` | `github.com/ibednov/go-lepsios/crypto` | AES-GCM, token hash, backup codes |
| `redis` | `github.com/ibednov/go-lepsios/redis` | Redis client from small Config |
| `migrate` | `github.com/ibednov/go-lepsios/migrate` | Pre-migrate `pg_dump` backup via `files.Adapter` |
| `apperr` | `github.com/ibednov/go-lepsios/apperr` | Structured app error + HTTP mapper hook |
| `audit` | `github.com/ibednov/go-lepsios/audit` | Actor-action audit log (`who did what`) |
| `billing` | `github.com/ibednov/go-lepsios/billing` | Payment intents, strategies (`admin_confirm`), subscription/purchase/entitlements |
| `featureflags` | `github.com/ibednov/go-lepsios/featureflags` | Per-subject flag key helpers (normalize/merge) |

Prefix rule: SaaS billing lives under `billing/*`. Future goods shop → `commerce/*` (never a bare `catalog`).

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

New modules without published tags need a temporary `replace` (or tag `billing/v0.1.0` / `featureflags/v0.1.0` and drop replace).

## Versioning

Semver tags per module: `log/v0.1.0`, `httpx/v0.1.0`, etc.
