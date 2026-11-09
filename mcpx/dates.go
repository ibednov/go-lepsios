package mcpx

import (
	"strings"
	"time"
)

// ParseDateFrom parses an inclusive range start. An empty value yields (nil, nil)
// meaning "no filter". A date-only value ("2006-01-02") maps to UTC midnight; an
// RFC3339 timestamp passes through verbatim. Anything else returns ErrInvalidDate.
func ParseDateFrom(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	if d, err := time.Parse("2006-01-02", v); err == nil {
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
		return &t, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t, nil
	}
	return nil, ErrInvalidDate
}

// ParseDateTo parses an inclusive range end. An empty value yields (nil, nil). A
// date-only value maps to the FOLLOWING UTC midnight (AddDate(0,0,1)) so the whole
// day is covered as an exclusive upper bound; an RFC3339 timestamp passes through
// verbatim. Anything else returns ErrInvalidDate.
func ParseDateTo(v string) (*time.Time, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, nil
	}
	if d, err := time.Parse("2006-01-02", v); err == nil {
		t := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
		return &t, nil
	}
	if t, err := time.Parse(time.RFC3339, v); err == nil {
		return &t, nil
	}
	return nil, ErrInvalidDate
}

// NextOffset computes the pagination cursor for the next page. It returns nil
// when hasMore is false. Otherwise it advances by the number of returned rows
// (offset+count); when count is zero it falls back to offset+limit so a caller
// that reports no count still advances by a full page.
func NextOffset(offset, count, limit int, hasMore bool) *int {
	if !hasMore {
		return nil
	}
	n := offset + count
	if count == 0 {
		n = offset + limit
	}
	return &n
}
