# exchange

Country → FX provider registry.

Providers expose `BaseCurrency()` (hub for `OfficialRate.BasePerUnit`):

- NBRB (`BY`) → BYN
- CBAR (`AZ`) → AZN (planned)

Depends on `github.com/ibednov/go-lepsios/currency`.
