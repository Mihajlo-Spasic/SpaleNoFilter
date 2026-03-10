package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/instagram-clone/user-service/internal/config"
)

type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

type IssueTokenRequest struct {
	UserID    uint64 `json:"user_id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	UserAgent string `json:"user_agent"`
	IPAddress string `json:"ip_address"`
}

type AuthClient interface {
	IssueTokens(req *IssueTokenRequest) (*TokenPair, error)
	RefreshTokens(refreshToken string) (*TokenPair, error)
	// RevokeToken blacklists a single access token (logout current device)
	RevokeToken(token string) error
	// InvalidateUserTokens revokes all refresh tokens for a user (logout-all / password change)
	InvalidateUserTokens(userID uint64) error
}

type authClient struct {
	baseURL        string
	internalSecret string
	httpClient     *http.Client
}

func NewAuthClient(cfg *config.Config) AuthClient {
	return &authClient{
		baseURL:        cfg.AuthServiceURL,
		internalSecret: cfg.InternalSecret,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
	}
}

func (c *authClient) IssueTokens(req *IssueTokenRequest) (*TokenPair, error) {
	body, _ := json.Marshal(req)
	httpReq, _ := http.NewRequest(http.MethodPost, c.baseURL+"/internal/auth/issue", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Secret", c.internalSecret)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("auth service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth service returned %d", resp.StatusCode)
	}

	var pair TokenPair
	if err := json.NewDecoder(resp.Body).Decode(&pair); err != nil {
		return nil, fmt.Errorf("decode auth response: %w", err)
	}
	return &pair, nil
}

func (c *authClient) RefreshTokens(refreshToken string) (*TokenPair, error) {
	body, _ := json.Marshal(map[string]string{"refresh_token": refreshToken})
	httpReq, _ := http.NewRequest(http.MethodPost, c.baseURL+"/api/v1/auth/refresh", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("auth service request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed: %d", resp.StatusCode)
	}

	var pair TokenPair
	json.NewDecoder(resp.Body).Decode(&pair)
	return &pair, nil
}

func (c *authClient) RevokeToken(token string) error {
	body, _ := json.Marshal(map[string]string{"token": token})
	httpReq, _ := http.NewRequest(http.MethodPost, c.baseURL+"/api/v1/auth/revoke", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err // best-effort; token will expire naturally
	}
	defer resp.Body.Close()
	return nil
}

func (c *authClient) InvalidateUserTokens(userID uint64) error {
	body, _ := json.Marshal(map[string]uint64{"user_id": userID})
	httpReq, _ := http.NewRequest(http.MethodPost, c.baseURL+"/internal/auth/invalidate-user", bytes.NewBuffer(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Internal-Secret", c.internalSecret)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
