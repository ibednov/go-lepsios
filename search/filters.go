package search

import (
	"fmt"
	"regexp"
	"strings"
)

var filterIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)

// QuoteFilterValue escapes a scalar string for common quoted filter syntaxes.
func QuoteFilterValue(value string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(value) + `"`
}

// FilterBuilder assembles safely quoted equality and boolean clauses without
// imposing domain-specific field names.
type FilterBuilder struct{ clauses []string }

func NewFilterBuilder() *FilterBuilder { return &FilterBuilder{} }

func (b *FilterBuilder) Equal(attribute, value string) error {
	if !filterIdentifierPattern.MatchString(attribute) {
		return fmt.Errorf("invalid filter attribute %q", attribute)
	}
	b.clauses = append(b.clauses, attribute+" = "+QuoteFilterValue(value))
	return nil
}

func (b *FilterBuilder) Any(attribute string, values []string) error {
	if !filterIdentifierPattern.MatchString(attribute) {
		return fmt.Errorf("invalid filter attribute %q", attribute)
	}
	if len(values) == 0 {
		return nil
	}
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, attribute+" = "+QuoteFilterValue(value))
	}
	b.clauses = append(b.clauses, "("+strings.Join(parts, " OR ")+")")
	return nil
}

func (b *FilterBuilder) Add(expression string) {
	if strings.TrimSpace(expression) != "" {
		b.clauses = append(b.clauses, "("+strings.TrimSpace(expression)+")")
	}
}

func (b *FilterBuilder) Build() []string { return append([]string(nil), b.clauses...) }
