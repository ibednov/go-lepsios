package phonepassword

import "errors"

var (
	errPhoneRequired    = errors.New("phone required")
	errPasswordRequired = errors.New("password required")
	errPasswordTooShort = errors.New("password min length 8")
)
