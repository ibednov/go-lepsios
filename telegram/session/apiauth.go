package session

import (
	"context"
	"errors"
)

// WithAPI runs call(accessToken). On unauthorized clears tokens, calls silentAuth, retries once.
func WithAPI(
	ctx context.Context,
	session *Session,
	unauthorized error,
	silentAuth func(context.Context, *Session) error,
	call func(accessToken string) error,
) error {
	if session == nil {
		return ErrUnauthorized
	}
	err := call(session.AccessToken)
	if err == nil || !errors.Is(err, unauthorized) {
		return err
	}

	session.AccessToken = ""
	session.RefreshToken = ""
	if authErr := silentAuth(ctx, session); authErr != nil {
		return authErr
	}
	return call(session.AccessToken)
}
