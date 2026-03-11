package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/social-service/internal/models"
	"github.com/instagram-clone/social-service/internal/service"
)

type SocialController struct {
	svc service.SocialService
}

func NewSocialController(svc service.SocialService) *SocialController {
	return &SocialController{svc: svc}
}

// POST /api/v1/social/follow
func (c *SocialController) Follow(ctx *gin.Context) {
	actorID := mustUserID(ctx)
	var req models.FollowRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.svc.SendFollowRequest(actorID, req.TargetUserID); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "follow request sent"})
}

// DELETE /api/v1/social/follow/:target_id
func (c *SocialController) Unfollow(ctx *gin.Context) {
	actorID := mustUserID(ctx)
	targetID := mustParamID(ctx, "target_id")
	if err := c.svc.Unfollow(actorID, targetID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "unfollowed"})
}

// POST /api/v1/social/follow-requests/respond
func (c *SocialController) RespondToRequest(ctx *gin.Context) {
	ownerID := mustUserID(ctx)
	var req models.RespondFollowRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.svc.RespondToFollowRequest(ownerID, req.FollowerID, req.Accept); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	msg := "request rejected"
	if req.Accept {
		msg = "request accepted"
	}
	ctx.JSON(http.StatusOK, gin.H{"message": msg})
}

// DELETE /api/v1/social/followers/:follower_id
func (c *SocialController) RemoveFollower(ctx *gin.Context) {
	ownerID := mustUserID(ctx)
	followerID := mustParamID(ctx, "follower_id")
	if err := c.svc.RemoveFollower(ownerID, followerID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "follower removed"})
}

// GET /api/v1/social/users/:user_id/followers
func (c *SocialController) GetFollowers(ctx *gin.Context) {
	userID := mustParamID(ctx, "user_id")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	list, err := c.svc.GetFollowers(userID, page, size)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, list)
}

// GET /api/v1/social/users/:user_id/following
func (c *SocialController) GetFollowing(ctx *gin.Context) {
	userID := mustParamID(ctx, "user_id")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	list, err := c.svc.GetFollowing(userID, page, size)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, list)
}

// GET /api/v1/social/follow-requests/pending
func (c *SocialController) GetPendingRequests(ctx *gin.Context) {
	userID := mustUserID(ctx)
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	list, err := c.svc.GetPendingRequests(userID, page, size)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, list)
}

// POST /api/v1/social/block
func (c *SocialController) Block(ctx *gin.Context) {
	actorID := mustUserID(ctx)
	var req models.BlockRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := c.svc.Block(actorID, req.TargetUserID); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "user blocked"})
}

// DELETE /api/v1/social/block/:target_id
func (c *SocialController) Unblock(ctx *gin.Context) {
	actorID := mustUserID(ctx)
	targetID := mustParamID(ctx, "target_id")
	if err := c.svc.Unblock(actorID, targetID); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "user unblocked"})
}

// GET /api/v1/social/blocked
func (c *SocialController) GetBlocked(ctx *gin.Context) {
	userID := mustUserID(ctx)
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	list, err := c.svc.GetBlocked(userID, page, size)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, list)
}

// GET /api/v1/social/status/:target_id
func (c *SocialController) GetStatus(ctx *gin.Context) {
	viewerID := mustUserID(ctx)
	targetID := mustParamID(ctx, "target_id")
	status, err := c.svc.GetStatus(viewerID, targetID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, status)
}

// POST /internal/social/can-view
func (c *SocialController) CanView(ctx *gin.Context) {
	var req models.CanViewRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	canView, err := c.svc.CanView(req.ViewerID, req.OwnerID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, models.CanViewResponse{CanView: canView})
}

// GET /internal/social/following-ids/:user_id
func (c *SocialController) GetFollowingIDs(ctx *gin.Context) {
	userID := mustParamID(ctx, "user_id")
	ids, err := c.svc.GetFollowingIDs(userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"following_ids": ids})
}

// GET /internal/social/is-blocked
func (c *SocialController) IsBlocked(ctx *gin.Context) {
	var req models.IsBlockedRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	status, err := c.svc.GetStatus(req.ActorID, req.TargetID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"is_blocking":    status.IsBlocking,
		"is_blocked_by":  status.IsBlockedBy,
		"either_blocked": status.IsBlocking || status.IsBlockedBy,
	})
}

func (c *SocialController) Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "service": "social"})
}

func mustUserID(ctx *gin.Context) uint64 {
	v, _ := ctx.Get("user_id")
	if v == nil {
		return 0
	}
	return v.(uint64)
}

func mustParamID(ctx *gin.Context, param string) uint64 {
	id, _ := strconv.ParseUint(ctx.Param(param), 10, 64)
	return id
}
