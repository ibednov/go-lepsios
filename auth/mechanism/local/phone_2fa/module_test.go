package phone2fa_test

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/ibednov/go-lepsios/auth/mechanism/local/phone_2fa"
	phonepassword "github.com/ibednov/go-lepsios/auth/mechanism/local/phone_password"
	"github.com/ibednov/go-lepsios/auth/provider"
	"github.com/ibednov/go-lepsios/auth/session"
	"github.com/ibednov/go-lepsios/auth/token"
)

type memoryStore struct {
	items   map[string]string
	revoked int
	saveErr error
}

func (s *memoryStore) SaveRefresh(_ context.Context, hash, userID string, _ time.Time) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	if s.items == nil {
		s.items = make(map[string]string)
	}
	s.items[hash] = userID
	return nil
}
func (s *memoryStore) FindUserByRefresh(_ context.Context, hash string) (string, error) {
	return s.items[hash], nil
}
func (s *memoryStore) RevokeRefresh(_ context.Context, hash string) error {
	delete(s.items, hash)
	s.revoked++
	return nil
}
func (s *memoryStore) RevokeAllForUser(_ context.Context, userID string) error {
	for hash, id := range s.items {
		if id == userID {
			delete(s.items, hash)
		}
	}
	return nil
}

type store struct {
	verified phone2fa.VerifiedUser
	err      error
}

func (s store) Check(context.Context, *gin.Context) (any, error)    { return nil, nil }
func (s store) Generate(context.Context, *gin.Context) (any, error) { return nil, nil }
func (s store) Verify(context.Context, *gin.Context) (phone2fa.VerifiedUser, error) {
	return s.verified, s.err
}

func TestCommitConflictRevokesPairAndDoesNotWriteCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessions := &memoryStore{}
	tokens := token.NewManager(token.Config{Secret: "test-secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	refresh := session.NewRefreshService(tokens, sessions, session.HashToken)
	module := phone2fa.New(provider.LocalPhoneTOTP, tokens, refresh, store{verified: phone2fa.VerifiedUser{
		VerifiedUser:  phonepassword.VerifiedUser{UserID: "user-1"},
		CommitSession: func(context.Context) error { return phone2fa.ErrCredentialCommitConflict },
	}}, phone2fa.WithTokenTransport(phonepassword.TokenTransport{Access: phonepassword.DeliverJSON, Refresh: phonepassword.DeliverCookie}))
	router := gin.New()
	module.RegisterRoutes(router.Group("/"))
	request := httptest.NewRequest("POST", "/verify", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 401 {
		t.Fatalf("status = %d, want 401; body: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Set-Cookie") != "" {
		t.Fatal("refresh cookie was set after credential commit conflict")
	}
	if strings.Contains(recorder.Body.String(), "access_token") || strings.Contains(recorder.Body.String(), "refresh_token") {
		t.Fatalf("token pair was included after credential commit conflict: %s", recorder.Body.String())
	}
	if sessions.revoked != 1 || len(sessions.items) != 0 {
		t.Fatalf("session state after conflict: revoked=%d active=%d", sessions.revoked, len(sessions.items))
	}
}

func TestInfrastructureCommitFailureIsServerErrorAndSkipsAfterCommit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessions := &memoryStore{}
	tokens := token.NewManager(token.Config{Secret: "test-secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	refresh := session.NewRefreshService(tokens, sessions, session.HashToken)
	called := false
	module := phone2fa.New(provider.LocalPhoneTOTP, tokens, refresh, store{verified: phone2fa.VerifiedUser{
		VerifiedUser:  phonepassword.VerifiedUser{UserID: "user-1"},
		CommitSession: func(context.Context) error { return errors.New("database unavailable") },
		AfterCommit:   func(context.Context) { called = true },
	}}, phone2fa.WithTokenTransport(phonepassword.TokenTransport{Access: phonepassword.DeliverJSON, Refresh: phonepassword.DeliverCookie}))
	router := gin.New()
	module.RegisterRoutes(router.Group("/"))
	request := httptest.NewRequest("POST", "/verify", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 500 {
		t.Fatalf("status = %d, want 500; body: %s", recorder.Code, recorder.Body.String())
	}
	if recorder.Header().Get("Set-Cookie") != "" {
		t.Fatal("refresh cookie was set after credential commit failure")
	}
	if strings.Contains(recorder.Body.String(), "access_token") || strings.Contains(recorder.Body.String(), "refresh_token") {
		t.Fatalf("token pair was included after credential commit failure: %s", recorder.Body.String())
	}
	if called {
		t.Fatal("AfterCommit ran after failed credential commit")
	}
	if sessions.revoked != 1 || len(sessions.items) != 0 {
		t.Fatalf("session state after commit failure: revoked=%d active=%d", sessions.revoked, len(sessions.items))
	}
}

func TestAfterCommitRunsOnlyAfterCredentialMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessions := &memoryStore{}
	tokens := token.NewManager(token.Config{Secret: "test-secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	refresh := session.NewRefreshService(tokens, sessions, session.HashToken)
	sequence := 0
	module := phone2fa.New(provider.LocalPhoneTOTP, tokens, refresh, store{verified: phone2fa.VerifiedUser{
		VerifiedUser: phonepassword.VerifiedUser{UserID: "user-1"},
		CommitSession: func(context.Context) error {
			sequence = 1
			return nil
		},
		AfterCommit: func(context.Context) {
			if sequence == 1 {
				sequence = 2
			}
		},
	}}, phone2fa.WithTokenTransport(phonepassword.TokenTransport{Access: phonepassword.DeliverJSON, Refresh: phonepassword.DeliverCookie}))
	router := gin.New()
	module.RegisterRoutes(router.Group(""))
	request := httptest.NewRequest("POST", "/verify", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	if sequence != 2 {
		t.Fatalf("callback sequence = %d, want commit then after-commit", sequence)
	}
	if recorder.Header().Get("Set-Cookie") == "" {
		t.Fatal("refresh cookie missing after successful credential mutation")
	}
}

func TestOrdinaryTOTPVerificationWithoutCredentialMutationSucceeds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessions := &memoryStore{}
	tokens := token.NewManager(token.Config{Secret: "test-secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	refresh := session.NewRefreshService(tokens, sessions, session.HashToken)
	module := phone2fa.New(provider.LocalPhoneTOTP, tokens, refresh, store{verified: phone2fa.VerifiedUser{
		VerifiedUser: phonepassword.VerifiedUser{UserID: "user-1"},
	}}, phone2fa.WithTokenTransport(phonepassword.TokenTransport{Access: phonepassword.DeliverJSON, Refresh: phonepassword.DeliverCookie}))
	router := gin.New()
	module.RegisterRoutes(router.Group(""))
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest("POST", "/verify", nil))
	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200; body: %s", recorder.Code, recorder.Body.String())
	}
	if len(sessions.items) != 1 {
		t.Fatalf("active sessions = %d, want 1", len(sessions.items))
	}
}

func TestSessionIssueFailureDoesNotCommitCredentialMutation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	sessions := &memoryStore{saveErr: errors.New("redis unavailable")}
	tokens := token.NewManager(token.Config{Secret: "test-secret", AccessTTL: time.Minute, RefreshTTL: time.Hour})
	refresh := session.NewRefreshService(tokens, sessions, session.HashToken)
	committed := false
	module := phone2fa.New(provider.LocalPhoneTOTP, tokens, refresh, store{verified: phone2fa.VerifiedUser{
		VerifiedUser: phonepassword.VerifiedUser{UserID: "user-1"},
		CommitSession: func(context.Context) error {
			committed = true
			return nil
		},
	}}, phone2fa.WithTokenTransport(phonepassword.TokenTransport{Access: phonepassword.DeliverJSON, Refresh: phonepassword.DeliverCookie}))
	router := gin.New()
	module.RegisterRoutes(router.Group(""))
	request := httptest.NewRequest("POST", "/verify", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != 500 {
		t.Fatalf("status = %d, want 500; body: %s", recorder.Code, recorder.Body.String())
	}
	if committed {
		t.Fatal("credential mutation ran after session issue failed")
	}
	if recorder.Header().Get("Set-Cookie") != "" {
		t.Fatal("refresh cookie was set after session issue failed")
	}
}
