# log

Structured logging with zerolog.

```go
import "github.com/ibednov/go-lepsios/log"

l, _ := log.Setup(log.Config{
	Env:         "local",
	ServiceName: "eco-back",
	Level:       "info",
	// VectorURL: "disabled", // optional; local/dev defaults to http://127.0.0.1:8687
})
ctx := log.WithContext(ctx, l)

// fluent (preferred)
log.FromContext(ctx).Info().Str("user_id", id).Msg("started")

// key-value pairs helper
log.InfoCtx(ctx, "started", "user_id", id)
```

Local dev with VictoriaLogs: `VectorURL` defaults to `http://127.0.0.1:8687` for `local`/`dev`/`development`.
Stdout stays human-readable; JSON copies go to Vector in parallel.
