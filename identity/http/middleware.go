package identityhttp

import (
	"github.com/ibednov/go-lepsios/auth/claims"
	"github.com/ibednov/go-lepsios/identity"
	"github.com/gin-gonic/gin"
)

// Mapper maps Principal to identity.User.
type Mapper func(principal claims.Principal) (identity.User, error)

type options struct {
	defaultKind identity.ActorKind
	mapper      Mapper
}

// Option configures FromJWT middleware.
type Option func(*options)

// WithDefaultKind sets ActorKind when token has no kind.
func WithDefaultKind(kind identity.ActorKind) Option {
	return func(o *options) {
		o.defaultKind = kind
	}
}

// WithMapper overrides default Principal → User mapping.
func WithMapper(mapper Mapper) Option {
	return func(o *options) {
		o.mapper = mapper
	}
}

// FromJWT maps claims.Principal from context to identity.User.
func FromJWT(opts ...Option) gin.HandlerFunc {
	o := options{}
	for _, opt := range opts {
		opt(&o)
	}
	return func(c *gin.Context) {
		principal, ok := claims.PrincipalFromContext(c.Request.Context())
		if !ok {
			c.Next()
			return
		}

		var user identity.User
		var err error
		if o.mapper != nil {
			user, err = o.mapper(principal)
		} else {
			user = identity.User{
				ID:       principal.UserID,
				Kind:     principal.Kind,
				Provider: identity.ProviderID(principal.Provider),
				Email:    principal.Email,
				Roles:    principal.Roles,
			}
		}
		if user.Kind == "" {
			user.Kind = o.defaultKind
		}
		if err != nil {
			c.Next()
			return
		}

		ctx := identity.SetUser(c.Request.Context(), user)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
