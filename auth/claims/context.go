package claims

import "context"

type ctxPrincipalKey struct{}

// SetPrincipal stores Principal in ctx.
func SetPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, ctxPrincipalKey{}, p)
}

// PrincipalFromContext returns Principal from ctx.
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	if ctx == nil {
		return Principal{}, false
	}
	p, ok := ctx.Value(ctxPrincipalKey{}).(Principal)
	return p, ok
}
