package mcpx

import "context"

// Identity is the authenticated principal bound to an MCP request. Scope is the
// normalized tool scope ("read"/"write"); Subject is an opaque user/subject id;
// Extra carries an optional product-specific payload (for example the product's
// own auth record) so a product can recover its domain identity from mcpx hooks.
type Identity struct {
	Scope   string
	Subject string
	Extra   any
}

// CanWrite reports whether this identity may call mutating (write-scoped) tools.
func (i Identity) CanWrite() bool { return NormalizeScope(i.Scope) == ScopeWrite }

type identityKey struct{}

// ContextWithIdentity returns a copy of ctx carrying the given Identity.
func ContextWithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, id)
}

// IdentityFromContext extracts the Identity stored by ContextWithIdentity. The
// boolean reports whether an Identity was present.
func IdentityFromContext(ctx context.Context) (Identity, bool) {
	id, ok := ctx.Value(identityKey{}).(Identity)
	return id, ok
}
