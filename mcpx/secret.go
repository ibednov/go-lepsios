package mcpx

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
)

// HashSecret returns the SHA-256 digest of the secret string as a 32-byte slice.
func HashSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

// VerifySecret reports whether secret matches the stored hash. The comparison is
// constant-time and length-guarded so neither timing nor length leaks the hash.
func VerifySecret(secret string, hash []byte) bool {
	actual := HashSecret(secret)
	return len(hash) == len(actual) && subtle.ConstantTimeCompare(actual, hash) == 1
}

// IssueSecret mints a new credential secret: 32 random bytes encoded with
// base64.RawURLEncoding, plus its HashSecret digest for storage.
func IssueSecret() (secret string, hash []byte, err error) {
	raw := make([]byte, 32)
	if _, err = rand.Read(raw); err != nil {
		return "", nil, err
	}
	secret = base64.RawURLEncoding.EncodeToString(raw)
	return secret, HashSecret(secret), nil
}
