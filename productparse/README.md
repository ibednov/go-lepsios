# productparse

Single-URL product link parsers (card / price / image). Catalog crawl lives in `shopparser`.

## Contracts
- `Strategy` — CanHandle + Parse
- `CardParser` — ParseCard
- `ProductImageParser` — ParseProductImageURL
- `Registry` — host routing + logging

## Host responsibilities
UA/policy, CRM parse-monitor gates, FX, wish/file attach, recipe UI (future).
Inject `browser.JSONCapturer` for wildberries/ozon.
