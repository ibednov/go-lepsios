package mcpx

import "errors"

// CodedError carries a stable, machine-readable error code back to the agent.
// Tool handlers return these so agents can branch on a documented vocabulary
// instead of free-form English.
type CodedError struct{ Code string }

// Error implements error, returning the bare code.
func (e CodedError) Error() string { return e.Code }

// Coded builds a CodedError for the given code.
func Coded(code string) error { return CodedError{Code: code} }

// CodeEntry maps a set of sentinel errors to one stable code. A TableMapper
// returns Code when any of Sentinels matches (via errors.Is).
type CodeEntry struct {
	Code      string
	Sentinels []error
}

// ErrorMapper translates a domain error into a stable tool error code. tool is
// the tool name, available for logging/metrics by implementations.
type ErrorMapper interface {
	MapError(err error, tool string) error
}

// ErrorMapperFunc adapts a plain function to the ErrorMapper interface.
type ErrorMapperFunc func(err error, tool string) error

// MapError calls the underlying function so an ErrorMapperFunc satisfies
// ErrorMapper.
func (f ErrorMapperFunc) MapError(err error, tool string) error {
	return f(err, tool)
}

// TableMapper builds an ErrorMapper from a sentinel->code table. The returned
// mapper: returns nil for a nil error; passes an already-coded error through
// unchanged; otherwise returns Coded(entry.Code) for the first entry whose
// sentinel matches via errors.Is; when nothing matches it calls onUnknown (if
// provided) or returns Coded("internal_error").
func TableMapper(table []CodeEntry, onUnknown func(err error, tool string) error) ErrorMapperFunc {
	return func(err error, tool string) error {
		if err == nil {
			return nil
		}
		var coded CodedError
		if errors.As(err, &coded) {
			return coded
		}
		for _, entry := range table {
			for _, sentinel := range entry.Sentinels {
				if errors.Is(err, sentinel) {
					return Coded(entry.Code)
				}
			}
		}
		if onUnknown != nil {
			return onUnknown(err, tool)
		}
		return Coded("internal_error")
	}
}

// ErrInvalidDate is returned by the date helpers for an unparseable value.
var ErrInvalidDate = errors.New("mcpx: invalid date")
