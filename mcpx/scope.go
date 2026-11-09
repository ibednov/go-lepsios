package mcpx

// Tool scopes. Read keys may only call read-only tools; write keys may also call
// mutating tools. Credentials issued before scopes existed default to read.
const (
	ScopeRead  = "read"
	ScopeWrite = "write"
)

// NormalizeScope coerces arbitrary input to a known scope, defaulting to read so
// an unset or unexpected value never silently grants write access.
func NormalizeScope(scope string) string {
	if scope == ScopeWrite {
		return ScopeWrite
	}
	return ScopeRead
}
