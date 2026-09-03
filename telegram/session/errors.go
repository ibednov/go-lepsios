package session

import "errors"

var (
	ErrSessionNotFound = errors.New("telegram session not found")
	ErrUnauthorized    = errors.New("telegram user is not authenticated")
	ErrAPIUnauthorized = errors.New("wishimi api unauthorized")
)
