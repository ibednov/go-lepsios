package keycloak

import "errors"

// ErrUserNotFound is returned when a Keycloak user is missing.
var ErrUserNotFound = errors.New("user not found")
