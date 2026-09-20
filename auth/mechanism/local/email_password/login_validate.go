package emailpassword

import (
	"fmt"
	"strings"
)

func (req *LoginRequest) Normalize() {
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Phone = strings.TrimSpace(req.Phone)
}

func (req LoginRequest) Validate() error {
	req.Normalize()
	if req.Password == "" {
		return fmt.Errorf("password required")
	}
	if len(req.Password) < 8 {
		return fmt.Errorf("password min length 8")
	}
	hasEmail := req.Email != ""
	hasPhone := req.Phone != ""
	if !hasEmail && !hasPhone {
		return fmt.Errorf("email or phone required")
	}
	if hasEmail && !strings.Contains(req.Email, "@") {
		return fmt.Errorf("invalid email")
	}
	return nil
}
