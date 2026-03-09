package controller

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/auth-service/internal/models"
	"github.com/instagram-clone/auth-service/internal/service"
)

type AuthController struct {
	authService service.AuthService
}

func NewAuthController(authService service.AuthService) *AuthController {
	return &AuthController{authService: authService}
}

// IssueToken - internal endpoint called by user-service after login/register
func (c *AuthController) IssueToken(ctx *gin.Context) {
	var req models.IssueTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	req.UserAgent = ctx.GetHeader("User-Agent")
	req.IPAddress = ctx.ClientIP()

	pair, err := c.authService.IssueTokenPair(&req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
		return
	}

	ctx.JSON(http.StatusOK, pair)
}

// ValidateToken - validates a JWT access token
func (c *AuthController) ValidateToken(ctx *gin.Context) {
	var req models.ValidateTokenRequest

	// Support both JSON body and Bearer header
	authHeader := ctx.GetHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		req.Token = strings.TrimPrefix(authHeader, "Bearer ")
	} else if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}

	resp, err := c.authService.ValidateAccessToken(req.Token)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "validation error"})
		return
	}

	if !resp.Valid {
		ctx.JSON(http.StatusUnauthorized, resp)
		return
	}

	ctx.JSON(http.StatusOK, resp)
}

// RefreshToken - rotates refresh token and issues new access token
func (c *AuthController) RefreshToken(ctx *gin.Context) {
	var req models.RefreshTokenRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pair, err := c.authService.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, pair)
}

// RevokeToken - blacklists a JWT
func (c *AuthController) RevokeToken(ctx *gin.Context) {
	var req models.RevokeTokenRequest

	authHeader := ctx.GetHeader("Authorization")
	if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
		req.Token = strings.TrimPrefix(authHeader, "Bearer ")
	} else if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}

	if err := c.authService.RevokeToken(req.Token); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "revoke failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "token revoked"})
}

// InvalidateUserTokens - revokes all refresh tokens for a user (e.g. password change)
func (c *AuthController) InvalidateUserTokens(ctx *gin.Context) {
	var req models.InvalidateUserRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.authService.InvalidateUserTokens(req.UserID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "invalidation failed"})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"message": "all user tokens invalidated"})
}

func (c *AuthController) Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "service": "auth"})
}
