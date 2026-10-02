# exchange

Country → FX provider registry.

Provider IDs are country-prefixed; legacy keys stay readable from cache/DB:

| Country | Provider | ID | Legacy ID | Base |
|---------|----------|----|-----------|------|
| BY | NBRB | `by_nbrb` | `nbrb` | BYN |
| AZ | CBAR | `az_cbar` | — | AZN |

`OfficialRate.BasePerUnit` is always relative to `Provider.BaseCurrency()`.

SQL cache column: `rate_to_base` (legacy name `rate_to_byn` via `sqlstore.LegacyRateColumn`).
