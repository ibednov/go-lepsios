# money

Parse and format money amounts stored as integer cents/kopecks.

```go
cents, err := money.Parse("1 230,55")
s := money.FormatCur(cents, "USD") // "1 230,55 USD"
```
