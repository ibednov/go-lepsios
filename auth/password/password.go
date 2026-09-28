package password

import (
	"errors"
	"unicode"
)

const (
	MinLength = 8
	MaxLength = 64
)

// ErrWeak is returned when a password is empty, too short, too long,
// or missing a letter or a digit.
var ErrWeak = errors.New("password is too weak")

// Validate requires 8–64 characters, at least one letter and one digit.
func Validate(raw string) error {
	n := 0
	letter := false
	digit := false
	for _, r := range raw {
		n++
		switch {
		case unicode.IsLetter(r):
			letter = true
		case unicode.IsDigit(r):
			digit = true
		}
	}
	if n < MinLength || n > MaxLength || !letter || !digit {
		return ErrWeak
	}
	return nil
}
