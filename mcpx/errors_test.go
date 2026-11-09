package mcpx

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	errSampleNotFound = errors.New("sample: not found")
	errSampleConflict = errors.New("sample: conflict")
)

func TestTableMapperMapsSentinel(t *testing.T) {
	t.Parallel()
	table := []CodeEntry{
		{Code: "not_found", Sentinels: []error{errSampleNotFound}},
		{Code: "conflict", Sentinels: []error{errSampleConflict}},
	}
	mapper := TableMapper(table, nil)

	require.Equal(t, "not_found", mapper.MapError(errSampleNotFound, "tool").Error())
	require.Equal(t, "conflict", mapper.MapError(errSampleConflict, "tool").Error())

	// Wrapped sentinel still matches via errors.Is.
	wrapped := fmt.Errorf("db call: %w", errSampleNotFound)
	require.Equal(t, "not_found", mapper.MapError(wrapped, "tool").Error())
}

func TestTableMapperUnknown(t *testing.T) {
	t.Parallel()
	// nil onUnknown => internal_error.
	mapper := TableMapper(nil, nil)
	require.Equal(t, "internal_error", mapper.MapError(errors.New("boom"), "tool").Error())

	// custom onUnknown wins.
	custom := TableMapper(nil, func(err error, tool string) error {
		return Coded("custom_" + tool)
	})
	require.Equal(t, "custom_mytool", custom.MapError(errors.New("boom"), "mytool").Error())
}

func TestTableMapperPassThroughCoded(t *testing.T) {
	t.Parallel()
	mapper := TableMapper([]CodeEntry{{Code: "x", Sentinels: []error{errSampleNotFound}}}, nil)
	coded := Coded("already_coded")
	require.Equal(t, coded, mapper.MapError(coded, "tool"))
	require.Equal(t, "already_coded", mapper.MapError(coded, "tool").Error())
}

func TestTableMapperNil(t *testing.T) {
	t.Parallel()
	mapper := TableMapper(nil, nil)
	require.NoError(t, mapper.MapError(nil, "tool"))
}

func TestErrorMapperFuncSatisfiesInterface(t *testing.T) {
	t.Parallel()
	var m ErrorMapper = TableMapper(nil, nil)
	require.NotNil(t, m)
}
