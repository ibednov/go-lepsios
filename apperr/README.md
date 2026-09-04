# apperr

Structured application error + HTTP mapping hook for go-lepsios services.

| Import path | Requires |
|-------------|----------|
| `github.com/ibednov/go-lepsios/apperr` | `httpx`, `rate_limit` |

## What's shared

- `Code` — stable machine-readable error codes.
- `Error` — code + i18n `MessageKey` + technical `Message` + optional `HTTPStatus` + wrapped cause.
- Constructors: `New`, `Wrap`, `WithMessageKey`, `WithHTTPStatus`.
- `Mapper(localize, defaultStatus)` — builds an `httpx/response.ErrorMapper`: maps
  `rate_limit.ErrTooManyAttempts` → 429 and typed `*Error` → its status/code/key.

Product error codes and i18n message keys stay in the consumer. `Mapper`'s
`localize` callback decouples the lib from any concrete i18n bundle.

## Usage (consumer HTTP handler)

```go
mapper := apperr.Mapper(func(key string) string {
    return i18n.LocalizeFromContext(c, key)
}, http.StatusBadRequest)
response.Handle(c, err, mapper, nil)
```