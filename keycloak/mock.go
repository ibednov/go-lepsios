package keycloak

import (
	"context"
	"errors"
	"sync"
)

// MockClient is an in-memory Keycloak client for tests.
type MockClient struct {
	RealmName string

	Users     map[string]string // email -> id
	UsersByID map[string]map[string]any
	RoleUsers map[string][]string // role -> user ids

	mu sync.Mutex
}

// NewMockClient creates a mock client.
func NewMockClient() *MockClient {
	return &MockClient{
		RealmName: "md-tests",
		Users:     map[string]string{},
		UsersByID: map[string]map[string]any{},
		RoleUsers: map[string][]string{},
	}
}

func (m *MockClient) Realm() string {
	return m.RealmName
}

func (m *MockClient) Login(_ context.Context, email, password string) (Tokens, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if password == "wrong" {
		return Tokens{}, errors.New("invalid credentials")
	}
	if _, ok := m.Users[email]; !ok {
		return Tokens{}, errors.New("invalid credentials")
	}
	return Tokens{
		AccessToken:      "access-" + email,
		ExpiresIn:        300,
		RefreshToken:     "refresh-" + email,
		RefreshExpiresIn: 3600,
	}, nil
}

func (m *MockClient) Refresh(_ context.Context, refreshToken string) (Tokens, error) {
	if refreshToken == "" || refreshToken == "expired" {
		return Tokens{}, errors.New("invalid refresh token")
	}
	return Tokens{
		AccessToken:      "access-refreshed",
		ExpiresIn:        300,
		RefreshToken:     refreshToken,
		RefreshExpiresIn: 3600,
	}, nil
}

func (m *MockClient) Logout(_ context.Context, _ string) error {
	return nil
}

func (m *MockClient) RegisterWithPasswordAndRole(_ context.Context, email, _, role string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if role != "teacher" && role != "student" {
		return "", errors.New("unsupported role")
	}
	userID := "kc-" + email
	m.Users[email] = userID
	m.UsersByID[userID] = map[string]any{
		"id":      userID,
		"email":   email,
		"enabled": true,
	}
	m.RoleUsers[role] = append(m.RoleUsers[role], userID)
	return userID, nil
}

func (m *MockClient) FindUserByEmail(_ context.Context, email string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	userID, ok := m.Users[email]
	return userID, ok, nil
}

func (m *MockClient) CanSendRequiredActionsEmail() bool {
	return true
}

func (m *MockClient) SendRequiredActionsEmail(_ context.Context, _ string, _ []RequiredAction) error {
	return nil
}

func (m *MockClient) UpdateUserAttributes(_ context.Context, userID string, attributes map[string][]string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.UsersByID[userID]
	if !ok {
		user = map[string]any{"id": userID}
		m.UsersByID[userID] = user
	}
	attrs, _ := user["attributes"].(map[string]any)
	if attrs == nil {
		attrs = map[string]any{}
	}
	for key, values := range attributes {
		list := make([]any, len(values))
		for i, v := range values {
			list[i] = v
		}
		attrs[key] = list
	}
	user["attributes"] = attrs
	return nil
}

func (m *MockClient) GetUserByID(_ context.Context, userID string) (map[string]any, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.UsersByID[userID]
	if !ok {
		return nil, false, nil
	}
	out := map[string]any{}
	for k, v := range user {
		out[k] = v
	}
	return out, true, nil
}

func (m *MockClient) ListUsersWithRealmRole(_ context.Context, role string) ([]map[string]any, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := m.RoleUsers[role]
	out := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		if user, ok := m.UsersByID[id]; ok {
			cp := map[string]any{}
			for k, v := range user {
				cp[k] = v
			}
			out = append(out, cp)
		}
	}
	return out, nil
}

func (m *MockClient) DeleteUser(_ context.Context, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.UsersByID, userID)
	for email, id := range m.Users {
		if id == userID {
			delete(m.Users, email)
		}
	}
	for role, ids := range m.RoleUsers {
		filtered := ids[:0]
		for _, id := range ids {
			if id != userID {
				filtered = append(filtered, id)
			}
		}
		m.RoleUsers[role] = filtered
	}
	return nil
}

func (m *MockClient) MergeAndPutUser(_ context.Context, userID string, patch map[string]any) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, ok := m.UsersByID[userID]
	if !ok {
		return ErrUserNotFound
	}
	for k, v := range patch {
		if v != nil {
			user[k] = v
		}
	}
	return nil
}
