# shopparser

Universal shop crawl adapters for product catalogs.

## Packages

- root — `Adapter`, `Registry`, discovery/fetch outcomes, `ProductRef` / `RawOffer`
- `httpclient` — rate-limited same-origin HTTP client + diagnostics
- `pricing` — FX helpers for offer prices
- `woocommerce` / `woocommerce/storeapi` — WooCommerce Store API
- `tildastore` — Tilda store JSON
- `sitemapjsonld` — sitemap + JSON-LD / HTML product pages
- `errors` — crawl/parse error types

## Config

Adapters take `AdapterConfig` JSON. WooCommerce category filters default to empty (all products). Host apps pass `include_category_keywords` / `exclude_category_keywords` / `include_category_ids`.

## Module

```
github.com/ibednov/go-lepsios/shopparser
```
