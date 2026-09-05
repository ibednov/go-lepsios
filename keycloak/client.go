package keycloak

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// RequiredAction is a Keycloak required action sent via email.
type RequiredAction string

const (
	RequiredActionUpdatePassword RequiredAction = "UPDATE_PASSWORD"
	RequiredActionVerifyEmail    RequiredAction = "VERIFY_EMAIL"
)

// Tokens is the OIDC token response.
type Tokens struct {
	AccessToken      string
	ExpiresIn        int64
	RefreshToken     string
	RefreshExpiresIn int64
}

// Client is the Keycloak integration contract.
type Client interface {
	Realm() string
	Login(ctx context.Context, email, password string) (Tokens, error)
	Refresh(ctx context.Context, refreshToken string) (Tokens, error)
	Logout(ctx context.Context, refreshToken string) error
	RegisterWithPasswordAndRole(ctx context.Context, email, password, role string) (string, error)
	FindUserByEmail(ctx context.Context, email string) (string, bool, error)
	CanSendRequiredActionsEmail() bool
	SendRequiredActionsEmail(ctx context.Context, userID string, actions []RequiredAction) error
	UpdateUserAttributes(ctx context.Context, userID string, attributes map[string][]string) error
	GetUserByID(ctx context.Context, userID string) (map[string]any, bool, error)
	ListUsersWithRealmRole(ctx context.Context, role string) ([]map[string]any, error)
	DeleteUser(ctx context.Context, userID string) error
	MergeAndPutUser(ctx context.Context, userID string, patch map[string]any) error
}

// Config holds Keycloak HTTP client settings.
type Config struct {
	Realm              string
	AdminURL           string
	TokenURL           string
	LogoutURL          string
	ClientID           string
	ClientSecret       string
	ActionRedirectURI  string
	ActionClientID     string
	ActionLifespanSecs int64
	// TokenScope is the OIDC scope for the password grant (e.g. "openid profile").
	TokenScope string
}

// HTTPClient talks to Keycloak Admin and OIDC endpoints.
type HTTPClient struct {
	http               *http.Client
	adminURL           string
	tokenURL           string
	logoutURL          string
	clientID           string
	clientSecret       string
	realm              string
	actionRedirectURI  string
	actionClientID     string
	actionLifespanSecs int64
	tokenScope         string
}

// NewHTTPClient builds a Keycloak HTTP client from config values.
func NewHTTPClient(cfg Config) *HTTPClient {
	return &HTTPClient{
		http:               &http.Client{Timeout: 15 * time.Second},
		realm:              cfg.Realm,
		adminURL:           strings.TrimRight(cfg.AdminURL, "/"),
		tokenURL:           cfg.TokenURL,
		logoutURL:          cfg.LogoutURL,
		clientID:           cfg.ClientID,
		clientSecret:       cfg.ClientSecret,
		actionRedirectURI:  cfg.ActionRedirectURI,
		actionClientID:     cfg.ActionClientID,
		actionLifespanSecs: cfg.ActionLifespanSecs,
		tokenScope:         cfg.TokenScope,
	}
}

func (c *HTTPClient) Realm() string {
	return c.realm
}

func (c *HTTPClient) CanSendRequiredActionsEmail() bool {
	return strings.TrimSpace(c.actionRedirectURI) != "" && strings.TrimSpace(c.actionClientID) != ""
}

type tokenResponse struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int64  `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int64  `json:"refresh_expires_in"`
}

func (c *HTTPClient) Login(ctx context.Context, email, password string) (Tokens, error) {
	return c.passwordGrant(ctx, email, password)
}

func (c *HTTPClient) Refresh(ctx context.Context, refreshToken string) (Tokens, error) {
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"refresh_token": {refreshToken},
	}
	return c.requestTokens(ctx, form)
}

func (c *HTTPClient) Logout(ctx context.Context, refreshToken string) error {
	form := url.Values{
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"refresh_token": {refreshToken},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.logoutURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("keycloak logout failed: %s %s", resp.Status, string(body))
	}
	return nil
}

func (c *HTTPClient) RegisterWithPasswordAndRole(ctx context.Context, email, password, role string) (string, error) {
	userID, err := c.createUser(ctx, email, password)
	if err != nil {
		return "", err
	}
	if err := c.assignRealmRole(ctx, userID, role); err != nil {
		return "", err
	}
	return userID, nil
}

func (c *HTTPClient) FindUserByEmail(ctx context.Context, email string) (string, bool, error) {
	adminToken, err := c.clientCredentialsToken(ctx)
	if err != nil {
		return "", false, err
	}

	endpoint, err := url.Parse(c.adminURL + "/users")
	if err != nil {
		return "", false, err
	}
	q := endpoint.Query()
	q.Set("email", email)
	q.Set("exact", "true")
	endpoint.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", false, fmt.Errorf("keycloak find user failed: %s %s", resp.Status, string(body))
	}

	var users []struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return "", false, err
	}
	if len(users) == 0 {
		return "", false, nil
	}
	return users[0].ID, true, nil
}

func (c *HTTPClient) SendRequiredActionsEmail(ctx context.Context, userID string, actions []RequiredAction) error {
	if !c.CanSendRequiredActionsEmail() {
		return fmt.Errorf("keycloak action email config is missing")
	}

	adminToken, err := c.clientCredentialsToken(ctx)
	if err != nil {
		return err
	}

	endpoint, err := url.Parse(fmt.Sprintf("%s/users/%s/execute-actions-email", c.adminURL, userID))
	if err != nil {
		return err
	}
	q := endpoint.Query()
	q.Set("client_id", c.actionClientID)
	q.Set("redirect_uri", c.actionRedirectURI)
	q.Set("lifespan", fmt.Sprintf("%d", c.actionLifespanSecs))
	endpoint.RawQuery = q.Encode()

	payload := make([]string, len(actions))
	for i, action := range actions {
		payload[i] = string(action)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint.String(), strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("keycloak action email failed: %s %s", resp.Status, string(respBody))
	}
	return nil
}

func (c *HTTPClient) passwordGrant(ctx context.Context, email, password string) (Tokens, error) {
	form := url.Values{
		"grant_type":    {"password"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
		"username":      {email},
		"password":      {password},
		"scope":         {c.tokenScope},
	}
	return c.requestTokens(ctx, form)
}

func (c *HTTPClient) requestTokens(ctx context.Context, form url.Values) (Tokens, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return Tokens{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.http.Do(req)
	if err != nil {
		return Tokens{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return Tokens{}, fmt.Errorf("keycloak token request failed: %s %s", resp.Status, string(body))
	}

	var parsed tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return Tokens{}, err
	}

	return Tokens(parsed), nil
}

func (c *HTTPClient) clientCredentialsToken(ctx context.Context) (string, error) {
	form := url.Values{
		"grant_type":    {"client_credentials"},
		"client_id":     {c.clientID},
		"client_secret": {c.clientSecret},
	}
	tokens, err := c.requestTokens(ctx, form)
	if err != nil {
		return "", err
	}
	return tokens.AccessToken, nil
}

func (c *HTTPClient) createUser(ctx context.Context, email, password string) (string, error) {
	adminToken, err := c.clientCredentialsToken(ctx)
	if err != nil {
		return "", err
	}

	payload := map[string]any{
		"username": email,
		"email":    email,
		"enabled":  true,
		"credentials": []map[string]any{
			{
				"type":      "password",
				"value":     password,
				"temporary": false,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.adminURL+"/users", strings.NewReader(string(body)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return "", fmt.Errorf("keycloak create user failed: %s %s", resp.Status, string(respBody))
	}

	location := resp.Header.Get("Location")
	parts := strings.Split(location, "/")
	return parts[len(parts)-1], nil
}

func (c *HTTPClient) assignRealmRole(ctx context.Context, userID, role string) error {
	adminToken, err := c.clientCredentialsToken(ctx)
	if err != nil {
		return err
	}

	roleURL := fmt.Sprintf("%s/roles/%s", c.adminURL, role)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, roleURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("realm role %q not found", role)
	}

	var roleData struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&roleData); err != nil {
		return err
	}

	assignURL := fmt.Sprintf("%s/users/%s/role-mappings/realm", c.adminURL, userID)
	payload, err := json.Marshal([]map[string]string{
		{"id": roleData.ID, "name": roleData.Name},
	})
	if err != nil {
		return err
	}

	assignReq, err := http.NewRequestWithContext(ctx, http.MethodPost, assignURL, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	assignReq.Header.Set("Authorization", "Bearer "+adminToken)
	assignReq.Header.Set("Content-Type", "application/json")

	assignResp, err := c.http.Do(assignReq)
	if err != nil {
		return err
	}
	defer func() { _ = assignResp.Body.Close() }()

	if assignResp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(assignResp.Body, 1024))
		return fmt.Errorf("assign realm role failed: %s %s", assignResp.Status, string(body))
	}
	return nil
}

func (c *HTTPClient) UpdateUserAttributes(ctx context.Context, userID string, attributes map[string][]string) error {
	adminToken, err := c.clientCredentialsToken(ctx)
	if err != nil {
		return err
	}

	userURL := fmt.Sprintf("%s/users/%s", c.adminURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("get user failed: %s %s", resp.Status, string(body))
	}

	var user map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return err
	}
	delete(user, "access")

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

	body, err := json.Marshal(user)
	if err != nil {
		return err
	}

	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, userURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	putReq.Header.Set("Authorization", "Bearer "+adminToken)
	putReq.Header.Set("Content-Type", "application/json")

	putResp, err := c.http.Do(putReq)
	if err != nil {
		return err
	}
	defer func() { _ = putResp.Body.Close() }()

	if putResp.StatusCode >= http.StatusBadRequest {
		respBody, _ := io.ReadAll(io.LimitReader(putResp.Body, 1024))
		return fmt.Errorf("keycloak update failed: %s %s", putResp.Status, string(respBody))
	}
	return nil
}

func (c *HTTPClient) GetUserByID(ctx context.Context, userID string) (map[string]any, bool, error) {
	adminToken, err := c.clientCredentialsToken(ctx)
	if err != nil {
		return nil, false, err
	}

	userURL := fmt.Sprintf("%s/users/%s", c.adminURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, false, fmt.Errorf("get user failed: %s %s", resp.Status, string(body))
	}

	var user map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, false, err
	}
	return user, true, nil
}

func (c *HTTPClient) ListUsersWithRealmRole(ctx context.Context, role string) ([]map[string]any, error) {
	adminToken, err := c.clientCredentialsToken(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/roles/%s/users", c.adminURL, url.PathEscape(role))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("list role users failed: %s %s", resp.Status, string(body))
	}

	var users []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return nil, err
	}
	return users, nil
}

func (c *HTTPClient) DeleteUser(ctx context.Context, userID string) error {
	adminToken, err := c.clientCredentialsToken(ctx)
	if err != nil {
		return err
	}

	userURL := fmt.Sprintf("%s/users/%s", c.adminURL, userID)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, userURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+adminToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("delete user failed: %s %s", resp.Status, string(body))
	}
	return nil
}

func (c *HTTPClient) MergeAndPutUser(ctx context.Context, userID string, patch map[string]any) error {
	user, found, err := c.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !found {
		return ErrUserNotFound
	}

	delete(user, "access")
	for key, value := range patch {
		if value != nil {
			user[key] = value
		}
	}

	adminToken, err := c.clientCredentialsToken(ctx)
	if err != nil {
		return err
	}

	body, err := json.Marshal(user)
	if err != nil {
		return err
	}

	userURL := fmt.Sprintf("%s/users/%s", c.adminURL, userID)
	putReq, err := http.NewRequestWithContext(ctx, http.MethodPut, userURL, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	putReq.Header.Set("Authorization", "Bearer "+adminToken)
	putReq.Header.Set("Content-Type", "application/json")

	putResp, err := c.http.Do(putReq)
	if err != nil {
		return err
	}
	defer func() { _ = putResp.Body.Close() }()

	if putResp.StatusCode >= http.StatusBadRequest {
		respBody, _ := io.ReadAll(io.LimitReader(putResp.Body, 1024))
		return fmt.Errorf("put user failed: %s %s", putResp.Status, string(respBody))
	}
	return nil
}
