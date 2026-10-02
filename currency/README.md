# currency

ISO 4217 codes, official-rate snapshot, and base-currency hub converter.

Rates are always “how many units of **base** for `Scale` units of `Code`”
(`BasePerUnit`). Hub examples: BYN (NBRB), AZN (CBAR).

```go
got, err := currency.Convert(10, currency.CNY, currency.BYN)

az := currency.NewConverter(currency.AZN, rates)
got, err = az.Convert(10, currency.USD, currency.AZN)
```

Supported: BYN, AZN, USD, EUR, RUB, KZT, CNY.
