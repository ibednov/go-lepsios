# exchange

Country → FX provider registry.

Provider packages and IDs are country-prefixed:

| Country | Package | ID | Legacy import | Base |
|---------|---------|----|---------------|------|
| BY | `exchange/by_nbrb` | `by_nbrb` | `exchange/nbrb` | BYN |
| AZ | `exchange/az_cbar` | `az_cbar` | — | AZN |

`OfficialRate.BasePerUnit` is always relative to `Provider.BaseCurrency()`.

Cache load tries `ID()` then `LegacyIDs()` (e.g. old `nbrb` rows still hit for BY).

SQL cache column: `rate_to_base` (legacy name via `sqlstore.LegacyRateColumn`).
