package mcpx

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseDateFrom(t *testing.T) {
	t.Parallel()

	from, err := ParseDateFrom("2026-01-15")
	require.NoError(t, err)
	require.NotNil(t, from)
	require.Equal(t, time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC), *from)

	n, err := ParseDateFrom("")
	require.NoError(t, err)
	require.Nil(t, n)

	ts, err := ParseDateFrom("2026-01-15T09:30:00Z")
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 1, 15, 9, 30, 0, 0, time.UTC), *ts)

	_, err = ParseDateFrom("15.01.2026")
	require.ErrorIs(t, err, ErrInvalidDate)
}

func TestParseDateTo(t *testing.T) {
	t.Parallel()

	// date-only => next UTC midnight (exclusive end).
	to, err := ParseDateTo("2026-01-15")
	require.NoError(t, err)
	require.NotNil(t, to)
	require.Equal(t, time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC), *to)

	n, err := ParseDateTo("")
	require.NoError(t, err)
	require.Nil(t, n)

	ts, err := ParseDateTo("2026-01-15T09:30:00Z")
	require.NoError(t, err)
	require.Equal(t, time.Date(2026, 1, 15, 9, 30, 0, 0, time.UTC), *ts)

	_, err = ParseDateTo("15.01.2026")
	require.ErrorIs(t, err, ErrInvalidDate)
}

func TestNextOffset(t *testing.T) {
	t.Parallel()
	require.Nil(t, NextOffset(0, 50, 50, false))

	got := NextOffset(0, 50, 50, true)
	require.NotNil(t, got)
	require.Equal(t, 50, *got)

	got = NextOffset(100, 25, 50, true)
	require.NotNil(t, got)
	require.Equal(t, 125, *got)

	// count==0 falls back to offset+limit.
	got = NextOffset(10, 0, 50, true)
	require.NotNil(t, got)
	require.Equal(t, 60, *got)
}
