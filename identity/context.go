package identity

import "context"

type ctxUserKey struct{}
type ctxAuthorKey struct{}

// ProviderID identifies how the actor authenticated (no import from auth module).
type ProviderID string

// ActorKind identifies the actor role in the system.
type ActorKind string

const (
	ActorAdmin     ActorKind = "admin"
	ActorModerator ActorKind = "moderator"
	ActorCustomer  ActorKind = "customer"
	ActorSystem    ActorKind = "system"
)

// User is the authenticated actor in request/job context.
type User struct {
	ID       string
	Kind     ActorKind
	Provider ProviderID
	Email    string
	Roles    []string
	Plan     string
	Features []string
}

// Author is a denormalized author snapshot for audit/display.
type Author struct {
	UserID string
	Kind   ActorKind
	Name   string
}

// SetUser stores User in ctx.
func SetUser(ctx context.Context, u User) context.Context {
	return context.WithValue(ctx, ctxUserKey{}, u)
}

// UserFromContext returns User from ctx.
func UserFromContext(ctx context.Context) (User, bool) {
	if ctx == nil {
		return User{}, false
	}
	u, ok := ctx.Value(ctxUserKey{}).(User)
	return u, ok
}

// MustUser returns User from ctx or panics.
func MustUser(ctx context.Context) User {
	u, ok := UserFromContext(ctx)
	if !ok {
		panic("identity: user not found in context")
	}
	return u
}

// SetAuthor stores Author in ctx.
func SetAuthor(ctx context.Context, a Author) context.Context {
	return context.WithValue(ctx, ctxAuthorKey{}, a)
}

// AuthorFromContext returns Author from ctx.
func AuthorFromContext(ctx context.Context) (Author, bool) {
	if ctx == nil {
		return Author{}, false
	}
	a, ok := ctx.Value(ctxAuthorKey{}).(Author)
	return a, ok
}

// SystemUser returns the system actor for background jobs.
func SystemUser(serviceName string) User {
	return User{
		ID:   "system:" + serviceName,
		Kind: ActorSystem,
	}
}

// WithSystemUser puts system actor into ctx.
func WithSystemUser(ctx context.Context, serviceName string) context.Context {
	return SetUser(ctx, SystemUser(serviceName))
}
