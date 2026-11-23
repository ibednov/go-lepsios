# crawler

Catalog **crawlers**: Discover product refs, Fetch raw offers.

Not single-URL wishlist parse — see sibling leaf `parser`.
Crawlers may call `parser` when a product page needs shared extract (optional).

## API
- `Adapter` — `Discover` / `Fetch`
- `Registry.RegisterFactory(kind, …)`
- kinds: `woocommerce_store_api`, `tilda_store`, `sitemap_jsonld`
- `httpclient`, `errors`, `pricing`

## Host owns
UA, category filters, FX, matching, schedule, DB outcomes.
