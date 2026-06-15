# identity

Actor context for handlers, usecases, and jobs (stdlib core).

```go
import "github.com/ibednov/go-lepsios/identity"

user, ok := identity.UserFromContext(ctx)
```

HTTP mapping: `identity/http` subpackage (`FromJWT` middleware).
