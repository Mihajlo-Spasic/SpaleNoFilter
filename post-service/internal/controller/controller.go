package controller

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/instagram-clone/post-service/internal/models"
	"github.com/instagram-clone/post-service/internal/service"
)

type PostController struct {
	svc service.PostService
}

func NewPostController(svc service.PostService) *PostController {
	return &PostController{svc: svc}
}

// POST /api/v1/posts  (multipart/form-data: files[], caption)
func (c *PostController) CreatePost(ctx *gin.Context) {
	userID := mustUserID(ctx)

	form, err := ctx.MultipartForm()
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "multipart form required"})
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "at least one file required"})
		return
	}

	caption := ctx.PostForm("caption")

	post, err := c.svc.CreatePost(userID, caption, files)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusCreated, post)
}

// GET /api/v1/posts/:post_id/owner
func (c *PostController) GetPostOwner(ctx *gin.Context) {
	postID := mustParamID(ctx, "post_id")
	ownerID, err := c.svc.GetPostOwnerInternal(postID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"user_id": ownerID})
}

// GET /api/v1/posts/:post_id
func (c *PostController) GetPost(ctx *gin.Context) {
	postID := mustParamID(ctx, "post_id")
	viewerID := mustUserID(ctx)

	post, err := c.svc.GetPost(postID, viewerID)
	if err != nil {
		ctx.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, post)
}

// PATCH /api/v1/posts/:post_id/caption
func (c *PostController) UpdateCaption(ctx *gin.Context) {
	userID := mustUserID(ctx)
	postID := mustParamID(ctx, "post_id")

	var req models.UpdateCaptionRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := c.svc.UpdateCaption(postID, userID, req.Caption); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "caption updated"})
}

// DELETE /api/v1/posts/:post_id/media/:media_id
func (c *PostController) DeleteMedia(ctx *gin.Context) {
	userID := mustUserID(ctx)
	postID := mustParamID(ctx, "post_id")
	mediaID := mustParamID(ctx, "media_id")

	if err := c.svc.DeleteMedia(postID, mediaID, userID); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "media removed"})
}

// DELETE /api/v1/posts/:post_id
func (c *PostController) DeletePost(ctx *gin.Context) {
	userID := mustUserID(ctx)
	postID := mustParamID(ctx, "post_id")

	if err := c.svc.DeletePost(postID, userID); err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"message": "post deleted"})
}

// GET /api/v1/users/:user_id/posts
func (c *PostController) GetUserPosts(ctx *gin.Context) {
	ownerID := mustParamID(ctx, "user_id")
	viewerID := mustUserID(ctx)
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "12"))

	result, err := c.svc.GetUserPosts(ownerID, viewerID, page, size)
	if err != nil {
		ctx.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, result)
}

// POST /internal/posts/by-user-ids  (body: {user_ids:[], page, page_size})
func (c *PostController) GetByUserIDs(ctx *gin.Context) {
	var body struct {
		UserIDs  []uint64 `json:"user_ids"`
		Page     int      `json:"page"`
		PageSize int      `json:"page_size"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := c.svc.GetPostsByUserIDs(body.UserIDs, body.Page, body.PageSize)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, result)
}

// POST /internal/posts/:post_id/like-count
func (c *PostController) IncrementLike(ctx *gin.Context) {
	postID := mustParamID(ctx, "post_id")
	var body struct {
		Delta int `json:"delta"`
	}
	ctx.ShouldBindJSON(&body)
	c.svc.IncrementLike(postID, body.Delta)
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /internal/posts/:post_id/comment-count
func (c *PostController) IncrementComment(ctx *gin.Context) {
	postID := mustParamID(ctx, "post_id")
	var body struct {
		Delta int `json:"delta"`
	}
	ctx.ShouldBindJSON(&body)
	c.svc.IncrementComment(postID, body.Delta)
	ctx.JSON(http.StatusOK, gin.H{"ok": true})
}

// POST /api/v1/upload/avatar  (multipart: file)
// Uploads a single image to MinIO and returns its URL.
// Used for profile picture uploads.
func (c *PostController) UploadAvatar(ctx *gin.Context) {
	userID := mustUserID(ctx)
	fh, err := ctx.FormFile("file")
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "file field required"})
		return
	}
	f, err := fh.Open()
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "cannot read file"})
		return
	}
	defer f.Close()
	url, err := c.svc.UploadFile(userID, f, fh)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{"url": url})
}

func (c *PostController) Health(ctx *gin.Context) {
	ctx.JSON(http.StatusOK, gin.H{"status": "ok", "service": "post"})
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
