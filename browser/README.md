# browser

Shared headless Chromium sidecar. Products call it over internal HTTP instead of embedding Playwright in an API image.

| Import path | Description |
|-------------|-------------|
| `github.com/ibednov/go-lepsios/browser` | Allowlist, remote client, Chromium client, HTTP server |

## HTTP API

- `GET /health` — `ok`
- `POST /internal/v1/fetch-in-page` — warmup URL, then `fetch(request_url)` in page context
- `POST /internal/v1/capture-json` — open page and capture the first response matching a Playwright glob

Bearer `BROWSER_TOKEN` when set. Hosts must match `BROWSER_ALLOWED_HOSTS` (SSRF allowlist).

```json
{"warmup_url":"https://ozon.by/product/x-1/","request_url":"https://ozon.by/api/composer-api.bx/page/json/v2?url=/product/x-1/"}
```

## Consumer

```go
remote := browser.NewRemote(os.Getenv("PARSER_BROWSER_URL"), os.Getenv("PARSER_BROWSER_TOKEN"), 60*time.Second)
snap, err := remote.FetchInPage(ctx, warmupURL, requestURL)
```

Do not import `Client` into an Alpine/API image. Use `RemoteClient`.

## Run

```bash
go run ./cmd/browser
```

Env (aliases `PARSER_BROWSER_*` work too):

| Key | Default |
|-----|---------|
| `BROWSER_LISTEN` | `:8090` |
| `BROWSER_TOKEN` | empty (open) |
| `BROWSER_HEADLESS` | `true` |
| `BROWSER_TIMEOUT` | `45s` |
| `BROWSER_WARMUP` | `8s` |
| `BROWSER_ALLOWED_HOSTS` | ozon / wildberries / wb |

Docker: `browser/Dockerfile` + `browser/docker-compose.yml`. Stable hostname: `lepsios-browser`.
