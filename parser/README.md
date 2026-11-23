**Tag:** `parser/v0.1.4` — recipes: emall, shopby, ozby, dns, vek21, steam, onliner, detmir (`html_jsonld` + `api_json`). Native: wildberries, ozon, taobao, ali1688.

# parser

Universal **single-URL** product parsers (title / price / image).

Not a catalog crawler — see sibling leaf `crawler`.

## API
- `Strategy` — `CanHandle` + `Parse`
- `CardParser` — `ParseCard` (no image required)
- `ProductImageParser` — `ParseProductImageURL`
- `Registry` — host routing
- `recipe` — config-driven kind `html_jsonld` (assemble without new Go packages)

## Host owns
UA defaults, CRM gates, FX, wish attach, native browser wiring (WB/Ozon).

## Native vs recipe
- **recipe**: JSON hosts + extractors (jsonld/og) — add shops without code
- **native/** packages: antibot, multi-hop API, store-specific quirks (WB, Ozon, …)
