package currency

import (
	"errors"
	"strings"
)

// Code — ISO 4217.
type Code string

const (
	BYN Code = "BYN"
	USD Code = "USD"
	RUB Code = "RUB"
	KZT Code = "KZT"
	EUR Code = "EUR"
	CNY Code = "CNY"
)

var (
	ErrUnknownCode     = errors.New("currency: unknown code")
	ErrUnsupportedPair = errors.New("currency: unsupported conversion pair")
)

var supported = map[Code]struct{}{
	BYN: {},
	USD: {},
	RUB: {},
	KZT: {},
	EUR: {},
	CNY: {},
}

func (c Code) String() string {
	return string(c)
}

func (c Code) IsValid() bool {
	_, ok := supported[c]
	return ok
}

func Parse(raw string) (Code, error) {
	code := Code(strings.ToUpper(strings.TrimSpace(raw)))
	if !code.IsValid() {
		return "", ErrUnknownCode
	}
	return code, nil
}

func MustParse(raw string) Code {
	code, err := Parse(raw)
	if err != nil {
		return BYN
	}
	return code
}

func Normalize(raw string, fallback Code) Code {
	code, err := Parse(raw)
	if err != nil {
		return fallback
	}
	return code
}

func Ptr(code Code) *string {
	if !code.IsValid() {
		return nil
	}
	s := code.String()
	return &s
}
