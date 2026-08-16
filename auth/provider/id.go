package provider

import (
	"errors"
	"fmt"
)

// ID is a stable authentication provider identifier.
type ID string

const (
	LocalEmailPassword ID = "local-email-password"
	LocalEmail2FA      ID = "local-email-2fa"
	LocalPhonePassword ID = "local-phone-password"
	Telegram           ID = "telegram"
	Keycloak           ID = "keycloak"
)

var all = []ID{
	LocalEmailPassword,
	LocalEmail2FA,
	LocalPhonePassword,
	Telegram,
	Keycloak,
}

// Parse validates and returns provider ID.
func Parse(s string) (ID, error) {
	id := ID(s)
	if err := id.Validate(); err != nil {
		return "", err
	}
	return id, nil
}

// Validate checks that ID is known.
func (id ID) Validate() error {
	for _, known := range all {
		if id == known {
			return nil
		}
	}
	return fmt.Errorf("unknown provider id: %q", id)
}

// All returns registered provider IDs.
func All() []ID {
	out := make([]ID, len(all))
	copy(out, all)
	return out
}

// ErrUnknownProvider is returned for invalid provider IDs.
var ErrUnknownProvider = errors.New("unknown provider")
