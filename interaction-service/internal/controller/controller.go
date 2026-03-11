package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/interaction-service/internal/models"
	"github.com/instagram-clone/interaction-service/internal/service"
)

type InteractionController struct {
	svc service.InteractionService
}

func NewInteractionController(svc service.InteractionService) *InteractionController {
	return &InteractionController{svc: svc}
}

// POST /api/v1/posts/:post_id/like
func (c *InteractionController) Like(ctx *gin.Context) {
	userID := mustUserID(ctx)
	postID := mustParamID(ctx, "post_id")
	if err := c.svc.Like(userID, postID); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "liked"})
}

// DELETE /api/v1/posts/:post_id/like
func (c *InteractionController) Unlike(ctx *gin.Context) {
	userID := mustUserID(ctx)
	postID := mustParamID(ctx, "post_id")
	if err := c.svc.Unlike(userID, postID); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "unliked"})
}

// GET /api/v1/posts/:post_id/liked
func (c *InteractionController) HasLiked(ctx *gin.Context) {
	userID := mustUserID(ctx)
	postID := mustParamID(ctx, "post_id")
	liked, _ := c.svc.HasLiked(userID, postID)
	ctx.JSON(http.StatusOK, gin.H{"liked": liked})
}

// POST /api/v1/posts/:post_id/comments
func (c *InteractionController) AddComment(ctx *gin.Context) {
	userID := mustUserID(ctx)
	postID := mustParamID(ctx, "post_id")

	var req models.CreateCommentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	comment, err := c.svc.AddComment(userID, postID, req.Body)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusCreated, comment)
}

// PATCH /api/v1/comments/:comment_id
func (c *InteractionController) UpdateComment(ctx *gin.Context) {
	userID := mustUserID(ctx)
	commentID := mustParamID(ctx, "comment_id")

	var req models.UpdateCommentRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.svc.UpdateComment(commentID, userID, req.Body); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "comment updated"})
}

// DELETE /api/v1/comments/:comment_id
func (c *InteractionController) DeleteComment(ctx *gin.Context) {
	userID := mustUserID(ctx)
	commentID := mustParamID(ctx, "comment_id")

	if err := c.svc.DeleteComment(commentID, userID); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "comment deleted"})
}

// GET /api/v1/posts/:post_id/comments
func (c *InteractionController) ListComments(ctx *gin.Context) {
	viewerID := mustUserID(ctx)
	postID := mustParamID(ctx, "post_id")
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))

	result, err := c.svc.ListComments(postID, viewerID, page, size)
	if err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, result)
}

func (c *InteractionController) Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "service": "interaction"})
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
