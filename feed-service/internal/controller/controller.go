package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/feed-service/internal/service"
)

type FeedController struct {
	svc service.FeedService
}

func NewFeedController(svc service.FeedService) *FeedController {
	return &FeedController{svc: svc}
}

// GET /api/v1/feed?page=1&page_size=20
func (c *FeedController) GetFeed(ctx *gin.Context) {
	userID := mustUserID(ctx)
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))

	feed, err := c.svc.GetFeed(userID, page, size)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, feed)
}

func (c *FeedController) Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "service": "feed"})
}

func mustUserID(ctx *gin.Context) uint64 {
	v, _ := ctx.Get("user_id")
	if v == nil {
		return 0
	}
	return v.(uint64)
}
