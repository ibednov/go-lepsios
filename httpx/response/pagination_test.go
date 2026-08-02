package response_test

import (
	"testing"

	"github.com/ibednov/go-lepsios/httpx/response"
	"github.com/stretchr/testify/require"
)

func TestNormalizePage(t *testing.T) {
	page, perPage := response.NormalizePage(0, 0)
	require.Equal(t, 1, page)
	require.Equal(t, 10, perPage)

	page, perPage = response.NormalizePage(2, 200)
	require.Equal(t, 2, page)
	require.Equal(t, 100, perPage)
}

func TestNewPagination(t *testing.T) {
	p := response.NewPagination(1, 10, 0)
	require.Equal(t, 0, p.TotalPages)

	p = response.NewPagination(1, 10, 99)
	require.Equal(t, 10, p.TotalPages)
	require.Equal(t, 99, p.TotalCount)
}

func TestOffset(t *testing.T) {
	require.Equal(t, 0, response.Offset(1, 10))
	require.Equal(t, 20, response.Offset(3, 10))
}
