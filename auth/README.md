# auth

JWT authentication, middleware, and login mechanisms.

```go
import (
    "github.com/ibednov/go-lepsios/auth/token"
    authmiddleware "github.com/ibednov/go-lepsios/auth/middleware"
    emailpassword "github.com/ibednov/go-lepsios/auth/mechanism/local/email_password"
    phonepassword "github.com/ibednov/go-lepsios/auth/mechanism/local/phone_password"
    phone2fa "github.com/ibednov/go-lepsios/auth/mechanism/local/phone_2fa"
)
```

`email_password` and `email_2fa` keep their legacy email-capable contracts. New phone-only consumers should use `phone_password` and `phone_2fa`; these request and response contracts do not expose email fields. The library does not access the database — wire the credential verifier / TOTP store and `session.Store` in the service.
