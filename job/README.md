# job

CLI job runner with graceful shutdown and system identity.

```go
import "github.com/ibednov/go-lepsios/job"

root.AddCommand(job.NewCommand(cfg, "expire-holds", "Expire holds", func(ctx context.Context) error {
    return svc.ExpireHolds(ctx)
}))
```
