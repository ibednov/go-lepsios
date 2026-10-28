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

## Credential generations

Applications that must invalidate every session when credentials change can opt in with `RefreshService.WithGeneration(currentGeneration)`. The callback reads the current generation from the authoritative account store; errors fail closed. The refresh store must additionally implement `session.GenerationStore`: save a generation alongside every refresh token and return it on lookup. For older stored sessions, return generation `0` so that an account increment invalidates them.

The credential verifier should capture `VerifiedUser.AuthGeneration` with the credential it verified. The login modules carry that value into `claims.AccessClaims`; `Issue` compares it with the current generation before saving a session. If a login began before a password reset, its stale captured generation causes issuance to fail. For other providers, a nil generation makes `Issue` load the current generation itself.

Wrap the application's access validator with `validator.WithSessionGeneration(base, currentGeneration)`. Put this wrapper outside any session lookup cache so every access request checks the authoritative generation. A legacy token without a generation claim is accepted only while the account generation is `0`; after a password reset increments it, the legacy token is rejected. A present but mismatched claim or a lookup failure is rejected. Increment the account generation in the same database transaction that changes the password; Redis token cleanup can then run independently.
