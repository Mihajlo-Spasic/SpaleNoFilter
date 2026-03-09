package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/instagram-clone/auth-service/internal/config"
	"github.com/instagram-clone/auth-service/internal/models"
	"github.com/instagram-clone/auth-service/internal/repository"
)

type AuthService interface {
	IssueTokenPair(req *models.IssueTokenRequest) (*models.TokenPair, error)
	ValidateAccessToken(tokenStr string) (*models.ValidateTokenResponse, error)
	RefreshAccessToken(refreshToken string) (*models.TokenPair, error)
	RevokeToken(tokenStr string) error
	InvalidateUserTokens(userID uint64) error
}

type authService struct {
	tokenRepo repository.TokenRepository
	cfg       *config.Config
}

func NewAuthService(repo repository.TokenRepository, cfg *config.Config) AuthService {
	return &authService{tokenRepo: repo, cfg: cfg}
}

// jwtClaims is the JWT standard claims struct
type jwtClaims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (s *authService) IssueTokenPair(req *models.IssueTokenRequest) (*models.TokenPair, error) {
	jti := uuid.New().String()
	now := time.Now()
	accessExpiry := now.Add(s.cfg.JWTExpiry)

	claims := jwtClaims{
		UserID:   req.UserID,
		Username: req.Username,
		Email:    req.Email,
		Role:     req.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiry),
			Issuer:    "instagram-clone-auth",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	// Generate secure refresh token
	rawRefresh, err := generateSecureToken(32)
	if err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}

	refreshHash := hashToken(rawRefresh)
	refreshExpiry := now.Add(s.cfg.RefreshExpiry)

	rt := &models.RefreshToken{
		UserID:    req.UserID,
		TokenHash: refreshHash,
		UserAgent: req.UserAgent,
		IPAddress: req.IPAddress,
		ExpiresAt: refreshExpiry,
	}

	if err := s.tokenRepo.SaveRefreshToken(rt); err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &models.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefresh,
		ExpiresAt:    accessExpiry,
		TokenType:    "Bearer",
	}, nil
}

func (s *authService) ValidateAccessToken(tokenStr string) (*models.ValidateTokenResponse, error) {
	claims, err := s.parseToken(tokenStr)
	if err != nil {
		return &models.ValidateTokenResponse{Valid: false}, nil
	}

	// Check blacklist
	blacklisted, err := s.tokenRepo.IsJTIBlacklisted(claims.ID)
	if err != nil {
		return nil, fmt.Errorf("check blacklist: %w", err)
	}
	if blacklisted {
		return &models.ValidateTokenResponse{Valid: false}, nil
	}

	return &models.ValidateTokenResponse{
		Valid:    true,
		UserID:   claims.UserID,
		Username: claims.Username,
		Email:    claims.Email,
		Role:     claims.Role,
	}, nil
}

func (s *authService) RefreshAccessToken(rawRefreshToken string) (*models.TokenPair, error) {
	hash := hashToken(rawRefreshToken)

	storedToken, err := s.tokenRepo.FindRefreshToken(hash)
	if err != nil {
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	if storedToken == nil {
		return nil, fmt.Errorf("refresh token not found or revoked")
	}
	if time.Now().After(storedToken.ExpiresAt) {
		return nil, fmt.Errorf("refresh token expired")
	}

	// Rotate: revoke old, issue new pair
	if err := s.tokenRepo.RevokeRefreshToken(hash); err != nil {
		return nil, fmt.Errorf("revoke old refresh token: %w", err)
	}

	// We need to re-issue with same user info - caller must pass it or we store it
	// Here we issue with stored user_id (minimal info); user-service can re-validate
	req := &models.IssueTokenRequest{
		UserID:    storedToken.UserID,
		Username:  "", // will be enriched by user-service if needed
		UserAgent: storedToken.UserAgent,
		IPAddress: storedToken.IPAddress,
	}

	return s.IssueTokenPair(req)
}

func (s *authService) RevokeToken(tokenStr string) error {
	claims, err := s.parseToken(tokenStr)
	if err != nil {
		return nil // already invalid
	}

	exp := claims.ExpiresAt.Time
	return s.tokenRepo.BlacklistJTI(claims.ID, exp)
}

func (s *authService) InvalidateUserTokens(userID uint64) error {
	return s.tokenRepo.RevokeAllUserTokens(userID)
}

func (s *authService) parseToken(tokenStr string) (*jwtClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &jwtClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}
	return claims, nil
}

func generateSecureToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
