package password

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	require.NoError(t, Validate("password1"))
	require.NoError(t, Validate("Пароль123"))
	require.ErrorIs(t, Validate(""), ErrWeak)
	require.ErrorIs(t, Validate("short1"), ErrWeak)
	require.ErrorIs(t, Validate("password"), ErrWeak)
	require.ErrorIs(t, Validate("12345678"), ErrWeak)
}
