package telegram

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
)

const (
	ChallengePurposeLogin = "login"
	ChallengePurposeLink  = "link"

	ChallengeStatusPending  = "pending"
	ChallengeStatusApproved = "approved"
	ChallengeStatusExpired  = "expired"
	ChallengeStatusConsumed = "consumed"

	DefaultChallengeTTL = 10 * time.Minute
)

// ChallengeView is the public challenge DTO returned by HTTP handlers (via Store).
type ChallengeView struct {
	ID        string    `json:"id"`
	Code      string    `json:"code"`
	Purpose   string    `json:"purpose"`
	Status    string    `json:"status"`
	ExpiresAt time.Time `json:"expires_at"`
}

// NormalizePurpose returns login/link or empty if invalid.
func NormalizePurpose(purpose string) string {
	switch strings.TrimSpace(strings.ToLower(purpose)) {
	case "", ChallengePurposeLogin:
		return ChallengePurposeLogin
	case ChallengePurposeLink:
		return ChallengePurposeLink
	default:
		return ""
	}
}

// RandomCode returns an uppercase hex code of length n (n<=32).
func RandomCode(n int) (string, error) {
	if n <= 0 {
		n = 8
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(buf)[:n]), nil
}

// IsChallengeActive reports whether challenge can still be approved.
func IsChallengeActive(status string, expiresAt time.Time, now time.Time) bool {
	return status == ChallengeStatusPending && now.Before(expiresAt)
}
