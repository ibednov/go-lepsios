# dates

Date parsing (`ДД.ММ.ГГГГ`) and named period boundaries (`today`/`week`/`month`/…).

```go
from, to := dates.Period("week", time.Now(), loc)
day, err := dates.ParseDay("05.09.2026", loc)
```
