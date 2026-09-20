package phone

import (
	"errors"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

var ErrInvalid = errors.New("invalid phone number")

func stripSpaces(raw string) string {
	return strings.ReplaceAll(strings.TrimSpace(raw), " ", "")
}

func parse(raw, defaultRegion string) (*phonenumbers.PhoneNumber, error) {
	raw = stripSpaces(raw)
	if raw == "" {
		return nil, ErrInvalid
	}
	region := defaultRegion
	if strings.HasPrefix(raw, "+") {
		region = ""
	}
	num, err := phonenumbers.Parse(raw, region)
	if err != nil {
		return nil, err
	}
	return num, nil
}

func NormalizeE164(raw string, defaultRegion string) (string, error) {
	num, err := parse(raw, defaultRegion)
	if err != nil {
		return "", err
	}
	if !phonenumbers.IsValidNumber(num) {
		return "", ErrInvalid
	}
	return phonenumbers.Format(num, phonenumbers.E164), nil
}

func IsValidE164(raw string, defaultRegion string) bool {
	_, err := NormalizeE164(raw, defaultRegion)
	return err == nil
}
