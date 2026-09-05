package keycloak

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenProfile is a Keycloak custom profile claim (OIDC-style nested object).
type TokenProfile struct {
	FirstName  *string `json:"first_name,omitempty"`
	MiddleName *string `json:"middle_name,omitempty"`
	LastName   *string `json:"last_name,omitempty"`
	Phone      *string `json:"phone,omitempty"`
	Group      *string `json:"group,omitempty"`
	Faculty    *string `json:"faculty,omitempty"`
}

// RealmAccess holds Keycloak realm roles.
type RealmAccess struct {
	Roles []string `json:"roles"`
}

// TokenClaims is the validated Keycloak access token payload.
type TokenClaims struct {
	Sub         string
	Email       string
	Profile     *TokenProfile
	RealmAccess *RealmAccess
	Features    map[string]any
}

type jwtClaims struct {
	Sub         string          `json:"sub"`
	Email       string          `json:"email"`
	Profile     *TokenProfile   `json:"profile"`
	RealmAccess *RealmAccess    `json:"realm_access"`
	Features    json.RawMessage `json:"features"`
	jwt.RegisteredClaims
}

// JWKSValidator validates Keycloak RS256 access tokens.
type JWKSValidator struct {
	http     *http.Client
	jwksURL  string
	issuer   string
	audience string

	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey
}

// NewJWKSValidator creates a Keycloak JWT validator.
func NewJWKSValidator(jwksURL, issuer, audience string) *JWKSValidator {
	return &JWKSValidator{
		http:     &http.Client{Timeout: 10 * time.Second},
		jwksURL:  jwksURL,
		issuer:   issuer,
		audience: audience,
		keys:     map[string]*rsa.PublicKey{},
	}
}

// Validate parses and verifies an RS256 access token.
func (v *JWKSValidator) Validate(ctx context.Context, rawToken string) (TokenClaims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	token, err := parser.ParseWithClaims(rawToken, &jwtClaims{}, func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("jwt missing kid")
		}
		return v.keyForKid(ctx, kid)
	})
	if err != nil {
		return TokenClaims{}, err
	}

	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return TokenClaims{}, errors.New("invalid token")
	}
	if claims.Issuer != v.issuer {
		return TokenClaims{}, fmt.Errorf("invalid issuer")
	}
	if !audienceMatches(claims.Audience, v.audience) {
		return TokenClaims{}, fmt.Errorf("invalid audience")
	}

	return TokenClaims{
		Sub:         claims.Sub,
		Email:       claims.Email,
		Profile:     claims.Profile,
		RealmAccess: claims.RealmAccess,
		Features:    parseFeaturesClaim(claims.Features),
	}, nil
}

func parseFeaturesClaim(raw json.RawMessage) map[string]any {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var asMap map[string]any
	if err := json.Unmarshal(raw, &asMap); err == nil {
		return asMap
	}
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil && asString != "" {
		if err := json.Unmarshal([]byte(asString), &asMap); err == nil {
			return asMap
		}
	}
	return nil
}

func audienceMatches(aud jwt.ClaimStrings, audience string) bool {
	if len(aud) == 0 {
		return true
	}
	for _, item := range aud {
		if item == audience {
			return true
		}
	}
	return false
}

func (v *JWKSValidator) keyForKid(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	v.mu.RUnlock()
	if ok {
		return key, nil
	}

	if err := v.refreshJWKS(ctx); err != nil {
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok = v.keys[kid]
	if !ok {
		return nil, fmt.Errorf("jwks missing kid %s", kid)
	}
	return key, nil
}

func (v *JWKSValidator) refreshJWKS(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}

	resp, err := v.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("fetch jwks failed: %s %s", resp.Status, string(body))
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return err
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" || jwk.Kid == "" {
			continue
		}
		key, err := rsaPublicKeyFromJWK(jwk.N, jwk.E)
		if err != nil {
			continue
		}
		keys[jwk.Kid] = key
	}
	if len(keys) == 0 {
		return errors.New("jwks has no rsa keys")
	}

	v.mu.Lock()
	v.keys = keys
	v.mu.Unlock()
	return nil
}

func rsaPublicKeyFromJWK(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(nStr)
	if err != nil {
		return nil, err
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(eStr)
	if err != nil {
		return nil, err
	}

	n := new(big.Int).SetBytes(nBytes)
	e := 0
	for _, b := range eBytes {
		e = e<<8 + int(b)
	}
	if e == 0 {
		return nil, errors.New("invalid rsa exponent")
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}
