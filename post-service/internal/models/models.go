package models

import "time"

type MediaType string

const (
	MediaTypeImage MediaType = "image"
	MediaTypeVideo MediaType = "video"
)

type PostMedia struct {
	ID        uint64    `json:"id"`
	PostID    uint64    `json:"post_id"`
	MediaKey  string    `json:"-"`
	MediaURL  string    `json:"media_url"`
	MediaType MediaType `json:"media_type"`
	Position  int       `json:"position"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type Post struct {
	ID           uint64       `json:"id"`
	UserID       uint64       `json:"user_id"`
	Caption      string       `json:"caption"`
	LikeCount    uint32       `json:"like_count"`
	CommentCount uint32       `json:"comment_count"`
	Media        []*PostMedia `json:"media"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type UpdateCaptionRequest struct {
	Caption string `json:"caption" binding:"max=2200"`
}

type DeleteMediaRequest struct {
	MediaID uint64 `json:"media_id" binding:"required"`
}

type PostSummary struct {
	ID           uint64    `json:"id"`
	UserID       uint64    `json:"user_id"`
	ThumbnailURL string    `json:"thumbnail_url"`
	LikeCount    uint32    `json:"like_count"`
	CommentCount uint32    `json:"comment_count"`
	MediaCount   int       `json:"media_count"`
	CreatedAt    time.Time `json:"created_at"`
}

type PaginatedPosts struct {
	Data     []*Post `json:"data"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
	HasMore  bool    `json:"has_more"`
}

type PaginatedSummaries struct {
	Data     []*PostSummary `json:"data"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	HasMore  bool           `json:"has_more"`
}
