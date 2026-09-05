# keycloak

HTTP Admin/OIDC client and JWKS RS256 validator for Keycloak.

```go
import "github.com/ibednov/go-lepsios/keycloak"

client := keycloak.NewHTTPClient(keycloak.Config{
    Realm:        "my-realm",
    AdminURL:     "https://kc.example/admin/realms/my-realm",
    TokenURL:     "https://kc.example/realms/my-realm/protocol/openid-connect/token",
    LogoutURL:    "https://kc.example/realms/my-realm/protocol/openid-connect/logout",
    ClientID:     "api",
    ClientSecret: "…",
    TokenScope:   "openid profile",
})

validator := keycloak.NewJWKSValidator(jwksURL, issuer, audience)
claims, err := validator.Validate(ctx, accessToken)
```

- `Client` / `HTTPClient` — password grant, refresh, logout, admin user CRUD helpers
- `JWKSValidator` — RS256 access tokens → library `TokenClaims` (no app-specific authctx)
- `MockClient` — in-memory client for tests

Password-grant `scope` is **not** hardcoded; set `Config.TokenScope` (e.g. `"openid md-tests-profile"`).
