# shopparser

Universal shop crawl adapters for product catalogs. **Product policy stays in the host app** (filters, UA, FX rates, matching, persistence).

Module: `github.com/ibednov/go-lepsios/shopparser`  
Latest tag: `shopparser/v0.1.1` (Go module version `v0.1.1`)  
Repo: `github.com/ibednov/go-lepsios` (leaf dir `shopparser/`)

Reference consumer: `tea-find-api` (`internal/domain/crawl/fx.go`, runner, migrations for outcome codes).

---

## For agents (other products)

### What this is

| Package | Role |
|---------|------|
| root | `Adapter`, `Registry`, `ProductRef`, `RawOffer`, discovery/fetch outcomes |
| `httpclient` | rate-limited HTTP, same-origin redirects, body caps, diagnostics |
| `errors` | `CrawlError`, `Transient` / `Blocked` / `ParseFailed` / `SkippedProduct` |
| `pricing` | minor units, currency normalize, weight parse, FX helpers |
| `woocommerce` | WooCommerce Store API (`Kind = "woocommerce_store_api"`) |
| `tildastore` | Tilda store JSON (`Kind = "tilda_store"`) |
| `sitemapjsonld` | sitemap + JSON-LD/HTML (`Kind = "sitemap_jsonld"`) |

**Not included:** DB schema, crawl runner, product matching, scheduling, admin UI. Host owns those.

### Wire-up (minimal)

```go
import (
	"github.com/ibednov/go-lepsios/shopparser"
	"github.com/ibednov/go-lepsios/shopparser/httpclient"
	"github.com/ibednov/go-lepsios/shopparser/sitemapjsonld"
	"github.com/ibednov/go-lepsios/shopparser/tildastore"
	"github.com/ibednov/go-lepsios/shopparser/woocommerce"
)

cli := httpclient.New(httpclient.Config{
	UserAgent:               "MyBot/1.0 (+https://example.com)",
	DefaultRPS:              1.0,
	MaxBodyBytes:            httpclient.DefaultMaxBodyBytes,
	SameOriginRedirectsOnly: true,
})

reg := shopparser.NewRegistry()
reg.RegisterFactory(woocommerce.Kind, func(s shopparser.SourceConfig) (shopparser.Adapter, error) {
	// inject product-specific defaults HERE when AdapterConfig is empty
	return woocommerce.New(cli, s.Code, s.BaseURL, s.AdapterConfig)
})
reg.RegisterFactory(tildastore.Kind, func(s shopparser.SourceConfig) (shopparser.Adapter, error) {
	return tildastore.New(cli, s.Code, s.BaseURL, s.AdapterConfig)
})
reg.RegisterFactory(sitemapjsonld.Kind, func(s shopparser.SourceConfig) (shopparser.Adapter, error) {
	return sitemapjsonld.New(cli, s.Code, s.BaseURL, s.RateLimitRPS, s.AdapterConfig)
})

adapter, err := reg.ForSource(shopparser.SourceConfig{
	Code: "shop-a", BaseURL: "https://shop.example", Adapter: woocommerce.Kind,
	Enabled: true, AdapterConfig: json.RawMessage(`{"version":1}`),
})
```

Crawl loop (prefer outcome helpers — empty catalog without evidence is `parse_failed`):

```go
discovery, err := shopparser.DiscoverResultFor(ctx, adapter)
// discovery.Refs, discovery.Outcome, discovery.Evidence

for _, ref := range discovery.Refs {
	fetched, err := shopparser.FetchResultFor(ctx, adapter, ref)
	// fetched.Offers ([]RawOffer), fetched.Outcome, fetched.Reason
	// filtered/discontinued/skipped → errors.Is(..., shopparser/errors.ErrSkippedProduct)
}
```

Optional: if adapter implements `DiagnosticAdapter`, on failure call `LastDiagnosticResponse()` and persist (bounded body + hash).

### go.mod (deploy) vs go.work (local only)

**Product `go.mod` — tagged require only. Path `replace` is forbidden** (breaks Docker `go mod download`).

```go
require github.com/ibednov/go-lepsios/shopparser v0.1.1
```

Local edit of the leaf: gitignored product `go.work`:

```
use (
  .
  ../../_shared/go-lepsios/shopparser
)
```

Dockerfile must use `GOWORK=off`. Gate before push: `GOWORK=off go mod download && GOWORK=off go build ./...`.

### Adapter kinds & config JSON

All configs: `"version": 1` unless noted. Validate with `ValidateConfig(baseURL, raw)` before save.

**`woocommerce_store_api`**

```json
{
  "version": 1,
  "include_category_keywords": ["herb"],
  "exclude_category_keywords": ["gift"],
  "include_category_ids": [12],
  "exclude_paths": ["/product/sample/"]
}
```

- Empty filters = **all products** (no tea/product defaults in the library).
- Host injects domain keywords in FX/factory when config is empty (see tea-find).

**`tilda_store`**

```json
{
  "version": 1,
  "storepart_uids": ["484700456321"],
  "currency": "RUB"
}
```

`storepart_uids` required. Currency allowlist is adapter-enforced (see ValidateConfig tests).

**`sitemap_jsonld`**

```json
{
  "version": 1,
  "sitemap_url": "https://shop.example/sitemap.xml",
  "product_path_prefixes": ["/product/"],
  "currency": "BYN",
  "include_category_hints": [],
  "exclude_category_hints": []
}
```

`New(..., rateLimitRPS, raw)` — per-source RPS from `SourceConfig.RateLimitRPS`.

### Core types (host persists / matches)

```go
type ProductRef struct {
	SourceCode string
	URL        string
	SourceID   string // shop id/sku when known
}

type RawOffer struct {
	Source, URL, SourceSKU, Title, Kind, PriceMode string
	KindConfidence float64
	KindNeedsReview bool
	KindReviewReason string
	Price decimal.Decimal
	Currency string
	WeightG *int
	WeightGrams *decimal.Decimal
	UnitHint UnitHint // g | piece | unknown
	InStock bool
	ImageURL, Brand string
	Attrs map[string]string
	ObservedAt time.Time
}
```

### Outcomes (wire strings — store these, not Go names)

| Constant | Wire | Meaning |
|----------|------|---------|
| `OutcomeOK` | `ok` | success with refs/offers |
| `OutcomeConfirmedEmpty` | `confirmed_empty` | empty catalog **with** evidence |
| `OutcomeFiltered` | `filtered` | intentionally out of scope |
| `OutcomeDiscontinued` | `discontinued` | gone from shop |
| `OutcomeSkipped` | `skipped` | skip without treat as hard fail |
| `OutcomeParseFailed` | `parse_failed` | bad HTML/JSON/contract |
| `OutcomeBlocked` | `blocked` | challenge / forbidden |
| `OutcomeTransient` | `transient` | retryable |
| `OutcomeTruncated` | `truncated` | body size / incomplete read |

If DB CHECK lists legacy values, migrate (tea-find: `00075_crawl_outcome_filtered.sql` renamed `filtered_non_tea` → `filtered`).

### Product policy (must stay in host)

- User-Agent, default RPS, schedule
- Category keywords / brand filters
- Currency FX (wrap `pricing.FX` / `FallbackFX` with live rates)
- Matching RawOffer → catalog product
- Persist runs, diagnostics, outcomes
- Custom adapters: implement `shopparser.Adapter` (+ optional `OutcomeAdapter`, `DiagnosticAdapter`), `RegisterFactory` or `NewRegistry(adapter)`

### Do not

- Hard-code product brands, tea keywords, or shop codes into the leaf
- Change outcome **wire** strings without a host DB migration
- Parse truncated diagnostic bodies as full pages
- Trust empty Discover as empty shop — need `OutcomeConfirmedEmpty` + evidence
- Self-import leaf root for entity types when extracting new code (define API types in root package)

### Extend the leaf

1. Copy/add under `go-lepsios/shopparser/`
2. Keep public API product-neutral; filters via JSON config
3. Tests + fixtures under `shopparser/testdata/`
4. `go test ./...` in leaf; bump tag `shopparser/v0.x.y`; push tag
5. Consumers bump require

### Local path (this machine)

`/Users/alepsios/dev/mine/_shared/go-lepsios/shopparser`
