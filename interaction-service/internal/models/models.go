package models

import "time"

type Like struct {
	ID        uint64    `json:"id"`
	UserID    uint64    `json:"user_id"`
	PostID    uint64    `json:"post_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Comment struct {
	ID        uint64    `json:"id"`
	PostID    uint64    `json:"post_id"`
	UserID    uint64    `json:"user_id"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateCommentRequest struct {
	Body string `json:"body" binding:"required,min=1,max=2200"`
}

type UpdateCommentRequest struct {
	Body string `json:"body" binding:"required,min=1,max=2200"`
}

type PaginatedComments struct {
	Data     []*Comment `json:"data"`
	Total    int64      `json:"total"`
	Page     int        `json:"page"`
	PageSize int        `json:"page_size"`
	HasMore  bool       `json:"has_more"`
}
