# auth

JWT authentication, middleware, and login mechanisms.

```go
import (
    "github.com/ibednov/go-lepsios/auth/token"
    authmiddleware "github.com/ibednov/go-lepsios/auth/middleware"
    emailpassword "github.com/ibednov/go-lepsios/auth/mechanism/local/email_password"
)
```

Lib does not access the database — wire `CredentialVerifier` and `session.Store` in the service.
