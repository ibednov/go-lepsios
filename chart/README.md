# chart

In-memory PNG charts for Telegram / bots (no disk writes).

```go
png, err := chart.HorizontalBars("Траты", []chart.Slice{
    {Label: "Еда", Value: 45000, Pct: 36},
    {Label: "Транспорт", Value: 28000, Pct: 22},
})
```

Embeds Noto Sans for Cyrillic labels.
