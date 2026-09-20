package emailpassword_test

import (
	"testing"

	emailpassword "github.com/ibednov/go-lepsios/auth/mechanism/local/email_password"
	"github.com/stretchr/testify/require"
)

func TestLoginRequestValidatePhoneFirst(t *testing.T) {
	t.Parallel()
	req := emailpassword.LoginRequest{Phone: "+375291234567", Password: "password1"}
	require.NoError(t, req.Validate())
}

func TestLoginRequestValidateRequiresIdentifier(t *testing.T) {
	t.Parallel()
	req := emailpassword.LoginRequest{Password: "password1"}
	require.Error(t, req.Validate())
}
