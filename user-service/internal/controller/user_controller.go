package controller

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/user-service/internal/models"
	"github.com/instagram-clone/user-service/internal/service"
)

type UserController struct {
	svc service.UserService
}

func NewUserController(svc service.UserService) *UserController {
	return &UserController{svc: svc}
}

// POST /api/v1/auth/register
func (c *UserController) Register(ctx *gin.Context) {
	var req models.RegisterRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := c.svc.Register(&req, ctx.GetHeader("User-Agent"), ctx.ClientIP())
	if err != nil {
		ctx.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, resp)
}

// POST /api/v1/auth/login
func (c *UserController) Login(ctx *gin.Context) {
	var req models.LoginRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := c.svc.Login(&req, ctx.GetHeader("User-Agent"), ctx.ClientIP())
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

// POST /api/v1/auth/refresh
func (c *UserController) RefreshToken(ctx *gin.Context) {
	var body struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	resp, err := c.svc.RefreshToken(body.RefreshToken)
	if err != nil {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, resp)
}

// POST /api/v1/auth/logout
func (c *UserController) Logout(ctx *gin.Context) {
	userID := mustUserID(ctx)
	token := ctx.GetString("access_token")
	if err := c.svc.Logout(userID, token); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "logout failed"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

// POST /api/v1/auth/logout-all
func (c *UserController) LogoutAll(ctx *gin.Context) {
	if err := c.svc.LogoutAll(mustUserID(ctx)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "logout-all failed"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "all sessions terminated"})
}

// PUT /api/v1/auth/change-password
func (c *UserController) ChangePassword(ctx *gin.Context) {
	var req models.ChangePasswordRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.svc.ChangePassword(mustUserID(ctx), &req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "password changed, all sessions terminated"})
}

// GET /api/v1/users/me
func (c *UserController) GetMyProfile(ctx *gin.Context) {
	user, err := c.svc.GetMyProfile(mustUserID(ctx))
	if err != nil || user == nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	ctx.JSON(http.StatusOK, user)
}

// GET /api/v1/users/:username/profile
// Returns profile data only. Social status (is_following, is_blocked, etc.)
// must be fetched from social-service by the caller.
func (c *UserController) GetPublicProfile(ctx *gin.Context) {
	profile, err := c.svc.GetPublicProfile(ctx.Param("username"))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, profile)
}

// PUT /api/v1/users/me
func (c *UserController) UpdateProfile(ctx *gin.Context) {
	var req models.UpdateProfileRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	user, err := c.svc.UpdateProfile(mustUserID(ctx), &req)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, user)
}

// PUT /api/v1/users/me/avatar
func (c *UserController) UpdateAvatar(ctx *gin.Context) {
	var req models.UpdateAvatarRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.svc.UpdateAvatar(mustUserID(ctx), req.AvatarURL); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update avatar"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "avatar updated"})
}

// DELETE /api/v1/users/me
func (c *UserController) DeleteAccount(ctx *gin.Context) {
	if err := c.svc.DeleteAccount(mustUserID(ctx)); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete account"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "account deleted"})
}

// GET /api/v1/users/search?q=john&page=1&page_size=20
func (c *UserController) SearchUsers(ctx *gin.Context) {
	var req models.SearchUsersRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	result, err := c.svc.SearchUsers(req.Query, req.Page, req.PageSize)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
		return
	}
	ctx.JSON(http.StatusOK, result)
}

// GET /api/v1/users/:id  (protected - requires JWT)
// Returns the public profile for any user by numeric ID.
// Used by the frontend when resolving usernames in list modals (followers/following/blocked).
func (c *UserController) GetPublicProfileByID(ctx *gin.Context) {
	var id uint64
	if _, err := fmt.Sscanf(ctx.Param("id"), "%d", &id); err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	profile, err := c.svc.GetPublicProfileByID(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, profile)
}

// GET /internal/users/:id
func (c *UserController) GetUserByID(ctx *gin.Context) {
	var id uint64
	if _, err := fmt.Sscanf(ctx.Param("id"), "%d", &id); err != nil || id == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid user id"})
		return
	}
	user, err := c.svc.GetUserByID(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, user)
}

// GET /internal/users/by-username/:username
func (c *UserController) GetUserByUsername(ctx *gin.Context) {
	user, err := c.svc.GetUserByUsername(ctx.Param("username"))
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, user)
}

// GET /internal/users/:id/is-private  - lightweight single-field check used by social-service
func (c *UserController) IsPrivate(ctx *gin.Context) {
	var id uint64
	fmt.Sscanf(ctx.Param("id"), "%d", &id)
	user, err := c.svc.GetUserByID(id)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"is_private": user.IsPrivate,
		"is_active":  user.IsActive,
	})
}

func (c *UserController) Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "service": "user"})
}

func mustUserID(ctx *gin.Context) uint64 {
	v, _ := ctx.Get("user_id")
	if v == nil {
		return 0
	}
	return v.(uint64)
}
