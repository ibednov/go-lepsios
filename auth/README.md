# auth

JWT authentication, middleware, and login mechanisms.

`recovery` provides product-neutral orchestration for one-time account recovery links. Product modules supply account lookup, quota reservation, persistence, mail composition and atomic credential mutation through typed ports. See [`recovery/README.md`](recovery/README.md) for the contract and [`recovery/ERRORS.md`](recovery/ERRORS.md) for error mapping guidance. The package tests include an independent opaque-account adapter example that does not depend on a product repository.

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

## Logout scope

`session.RegisterRefreshLogout` defaults to `session.LogoutAllSessions` to preserve its historical behavior. To make logout local to the authenticated refresh session, opt in with `session.WithLogoutScope(session.LogoutCurrentSession)`. This revokes the session named by the validated access token's `SessionID` claim and rejects a token without that claim; it never falls back to revoking every session. `RefreshService.Issue` binds `SessionID` to the stored refresh-token hash, so the application's validator must preserve and validate that claim before using this scope.

```go
session.RegisterRefreshLogout(
	meGroup,
	refreshService,
	validator,
	session.WithTokenTransport(session.TokenTransport{Refresh: session.DeliverCookie}),
	session.WithLogoutScope(session.LogoutCurrentSession),
)
```

Keep the default scope for APIs whose logout contract intentionally signs out every device. Use the current-session scope when separate clients or browser origins own independent refresh sessions.

## Client-aware sessions

`claims.AccessClaims.ClientID` is an optional client marker (`web`, `ios`, `android`, `crm`, `cli`, or an application-defined identifier). It is omitted on legacy tokens. Login modules can assign a trusted server-side ID with `WithClientID`; the option overrides a callback's `VerifiedUser.ClientID` and does not read request input. For example, create separate module instances for product routes:

```go
crmLogin := email2fa.New(providerID, tokens, refresh, crmStore,
    email2fa.WithClientID("crm"),
    email2fa.WithTokenTransport(emailpassword.TokenTransport{
        Refresh: emailpassword.DeliverCookie,
        RefreshCookieName: "crm_refresh_token",
    }),
)
```

Pass the same `RefreshCookieName` through `session.TokenTransport` on that client's refresh/logout route. An empty name keeps the historical `refresh_token` cookie. If a named cookie is absent, the refresh route accepts the old cookie once and issues a rotated token in the configured cookie. `session.WithClientID` assigns the server-side client ID when rotating an old refresh session that has no stored marker. The first refresh after rollout therefore upgrades newly issued access and refresh credentials without requiring a fresh login.

The existing `session.Store` contract remains unchanged. To persist client markers and support client-scoped logout, an adapter also implements `session.ClientStore`. Its `FindRefreshClientID` returns an empty ID for legacy sessions. Client-aware issuance fails closed if the configured store cannot persist the marker. When `WithGeneration` is also enabled, the adapter must implement `session.ClientGenerationStore` and save the client ID and generation atomically. `session.WithLogoutScope(session.LogoutClientSessions)` revokes only sessions for the validated token's client. A legacy access token without a client ID falls back to revoking only its validated `SessionID`; this fallback does not require route client configuration and never broadens to account-wide logout.

Existing `Store` implementations, untagged claims and default cookie names remain compatible. Applications must configure distinct IDs/cookie names per client where sessions share an API host, and must not derive client IDs from untrusted request headers or body fields.

For an existing unmarked logout endpoint that must keep working after clients are introduced, `session.WithLogoutScope(session.LogoutUnmarkedSessions)` revokes only unmarked sessions when the store implements `ClientStore`. This protects marked client sessions from an old endpoint's account-wide logout. With an old `Store` that lacks client metadata support, it retains the historical all-session behavior.

### 2FA credential commit contract and errors

`phone_2fa.VerifiedUser.CommitSession` lets an application defer one-time credential changes until `RefreshService.Issue` has created a session. The module invokes that callback before disclosing access or refresh credentials. When a callback returns `ErrCredentialCommitConflict`, the module revokes the newly issued refresh session and responds as a failed verification. Other callback errors also revoke the undisclosed session, then return an internal error. If cleanup itself fails, the module logs the cleanup failure without logging a token and returns an internal error. `AfterCommit` runs only after the credential mutation succeeds and before tokens are delivered; use it for best-effort audit/notification side effects.

| `error` | Source and trigger | HTTP | Safe message | Client behavior | Retry and trace |
| --- | --- | --- | --- | --- | --- |
| `error.auth.2fa.verify_failed` | `mechanism/local/phone_2fa/module.go`: invalid verification or one-time credential conflict | 401 | Unable to verify the 2FA code | Enter a valid TOTP/backup code | No automatic retry; correlate by request ID |
| `error.auth.2fa.credential_commit_failed` | Same module: credential persistence failed after session issue; session cleanup succeeded | 500 | Unable to complete 2FA verification | Retry with a current code after service recovery | Retry is safe; correlate by request ID and `auth.2fa.credential_commit_failed` log event |
| `error.auth.2fa.session_cleanup_failed` | Same module: revocation of the undisclosed newly issued session failed | 500 | Unable to complete 2FA verification | Do not treat the operation as successful; retry after service recovery | No client credential was disclosed; correlate by request ID and `auth.2fa.session_cleanup_failed` log event |

Responses keep the standard envelope (`error`, `message`, `code`). Tokens and one-time codes are never logged.
