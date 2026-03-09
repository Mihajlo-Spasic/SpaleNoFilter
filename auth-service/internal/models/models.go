package models

import "time"

// RefreshToken represents a stored refresh token record
type RefreshToken struct {
	ID        uint64     `json:"id"`
	UserID    uint64     `json:"user_id"`
	TokenHash string     `json:"-"`
	UserAgent string     `json:"user_agent"`
	IPAddress string     `json:"ip_address"`
	ExpiresAt time.Time  `json:"expires_at"`
	Revoked   bool       `json:"revoked"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// TokenBlacklist represents a blacklisted JWT (by jti)
type TokenBlacklist struct {
	ID        uint64    `json:"id"`
	JTI       string    `json:"jti"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// TokenPair is the response returned on successful auth
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
	TokenType    string    `json:"token_type"`
}

// Claims is the JWT payload
type Claims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	JTI      string `json:"jti"`
}

// IssueTokenRequest - internal request from user-service
type IssueTokenRequest struct {
	UserID    uint64 `json:"user_id" binding:"required"`
	Username  string `json:"username" binding:"required"`
	Email     string `json:"email" binding:"required"`
	Role      string `json:"role"`
	UserAgent string `json:"user_agent"`
	IPAddress string `json:"ip_address"`
}

type ValidateTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type RevokeTokenRequest struct {
	Token string `json:"token" binding:"required"`
}

type InvalidateUserRequest struct {
	UserID uint64 `json:"user_id" binding:"required"`
}

type ValidateTokenResponse struct {
	Valid    bool   `json:"valid"`
	UserID   uint64 `json:"user_id,omitempty"`
	Username string `json:"username,omitempty"`
	Email    string `json:"email,omitempty"`
	Role     string `json:"role,omitempty"`
}
