package middleware

type options struct {
	skipPaths []string
}

// Option configures auth middleware.
type Option func(*options)

// WithSkipPaths skips middleware for matching path prefixes.
func WithSkipPaths(paths ...string) Option {
	return func(o *options) {
		o.skipPaths = append(o.skipPaths, paths...)
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
