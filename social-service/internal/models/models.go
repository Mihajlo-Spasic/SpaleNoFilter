package models

import "time"

// ─── Domain ─────────────────────────────────────────────────────────────────

type FollowStatus string

const (
	FollowStatusPending  FollowStatus = "pending"
	FollowStatusAccepted FollowStatus = "accepted"
)

type Follow struct {
	ID          uint64       `json:"id"`
	FollowerID  uint64       `json:"follower_id"`
	FollowingID uint64       `json:"following_id"`
	Status      FollowStatus `json:"status"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

type Block struct {
	ID        uint64    `json:"id"`
	BlockerID uint64    `json:"blocker_id"`
	BlockedID uint64    `json:"blocked_id"`
	CreatedAt time.Time `json:"created_at"`
}

type FollowRequest struct {
	TargetUserID uint64 `json:"target_user_id" binding:"required"`
}

type RespondFollowRequest struct {
	FollowerID uint64 `json:"follower_id" binding:"required"`
	Accept     bool   `json:"accept"`
}

type BlockRequest struct {
	TargetUserID uint64 `json:"target_user_id" binding:"required"`
}

type FollowEntry struct {
	UserID    uint64    `json:"user_id"`
	Status    string    `json:"status,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type SocialStatusResponse struct {
	IsFollowing  bool   `json:"is_following"`
	FollowStatus string `json:"follow_status,omitempty"` // pending | accepted
	IsFollowedBy bool   `json:"is_followed_by"`
	IsBlocking   bool   `json:"is_blocking"`
	IsBlockedBy  bool   `json:"is_blocked_by"`
}

type PaginatedFollowList struct {
	Data     []FollowEntry `json:"data"`
	Total    int64         `json:"total"`
	Page     int           `json:"page"`
	PageSize int           `json:"page_size"`
	HasMore  bool          `json:"has_more"`
}

type IsBlockedRequest struct {
	ActorID  uint64 `json:"actor_id"`
	TargetID uint64 `json:"target_id"`
}

type CanViewRequest struct {
	ViewerID uint64 `json:"viewer_id"`
	OwnerID  uint64 `json:"owner_id"`
}

type CanViewResponse struct {
	CanView bool `json:"can_view"`
}
