**Tag:** `parser/v0.1.7` — recipes: emall, shopby, ozby, dns, vek21, steam, onliner, detmir (`html_jsonld` + `api_json`). Overlay: `recipe.Merge` / `ParseConfigs` / `LoadConfigsFile`. Native antibot: wildberries, ozon, taobao, ali1688.

# parser

Universal **single-URL** product parsers (title / price / image).

Not a catalog crawler — see sibling leaf `crawler`.

## API
- `Strategy` — `CanHandle` + `Parse`
- `CardParser` — `ParseCard` (no image required)
- `ProductImageParser` — `ParseProductImageURL`
- `Registry` — host routing
- `recipe` — config-driven `html_jsonld` + `api_json` (Builtin + host overlay JSON)

## Host owns
UA defaults, CRM gates, FX, wish attach, native browser wiring (WB/Ozon), optional extra recipe JSON.

## Native vs recipe
- **recipe**: JSON hosts + extractors — add shops without code (`Merge(Builtin(), extras...)`)
- **native/** packages: antibot multi-hop (WB, Ozon, taobao, 1688)

## Live checks
`go test ./parser -run Live` (skip with `-short` or `PARSER_LIVE=0`)
