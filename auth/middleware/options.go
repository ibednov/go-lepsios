package middleware

import "github.com/gin-gonic/gin"

type options struct {
	skipPaths      []string
	unauthorizedFn func(*gin.Context, string)
}

// Option configures auth middleware.
type Option func(*options)

// WithSkipPaths skips middleware for matching path prefixes.
func WithSkipPaths(paths ...string) Option {
	return func(o *options) {
		o.skipPaths = append(o.skipPaths, paths...)
	}
}

// WithUnauthorizedHandler lets an application own the response for auth
// failures, for example to apply its API error catalog and request locale.
func WithUnauthorizedHandler(handler func(*gin.Context, string)) Option {
	return func(o *options) {
		o.unauthorizedFn = handler
	}
}

func applyOptions(opts []Option) options {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

func shouldSkip(path string, skipPaths []string) bool {
	for _, p := range skipPaths {
		if p == "" {
			continue
		}
		if path == p || len(path) > len(p) && path[:len(p)] == p {
			return true
		}
	}
	return false
}
