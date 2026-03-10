package models

import "time"

type User struct {
	ID             uint64     `json:"id"`
	Username       string     `json:"username"`
	Email          string     `json:"email,omitempty"`
	PasswordHash   string     `json:"-"`
	FullName       string     `json:"full_name"`
	Bio            string     `json:"bio"`
	AvatarURL      string     `json:"avatar_url"`
	Website        string     `json:"website"`
	IsPrivate      bool       `json:"is_private"`
	IsVerified     bool       `json:"is_verified"`
	IsActive       bool       `json:"is_active"`
	Role           string     `json:"role"`
	PostCount      uint32     `json:"post_count"`
	FollowerCount  uint32     `json:"follower_count"`
	FollowingCount uint32     `json:"following_count"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"-"`
}

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=3,max=30,alphanum"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"max=100"`
}

type LoginRequest struct {
	UsernameOrEmail string `json:"username_or_email" binding:"required"`
	Password        string `json:"password"          binding:"required"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}

type UpdateProfileRequest struct {
	FullName  *string `json:"full_name"`
	Bio       *string `json:"bio"`
	Website   *string `json:"website"`
	IsPrivate *bool   `json:"is_private"`
}

type UpdateAvatarRequest struct {
	AvatarURL string `json:"avatar_url" binding:"required,url"`
}

type UserPublicProfile struct {
	ID             uint64    `json:"id"`
	Username       string    `json:"username"`
	FullName       string    `json:"full_name"`
	Bio            string    `json:"bio"`
	AvatarURL      string    `json:"avatar_url"`
	Website        string    `json:"website"`
	IsPrivate      bool      `json:"is_private"`
	IsVerified     bool      `json:"is_verified"`
	PostCount      uint32    `json:"post_count"`
	FollowerCount  uint32    `json:"follower_count"`
	FollowingCount uint32    `json:"following_count"`
	CreatedAt      time.Time `json:"created_at"`
}

type AuthResponse struct {
	User         *UserPublicProfile `json:"user"`
	AccessToken  string             `json:"access_token"`
	RefreshToken string             `json:"refresh_token"`
	ExpiresAt    time.Time          `json:"expires_at"`
	TokenType    string             `json:"token_type"`
}

type SearchUsersRequest struct {
	Query    string `form:"q"         binding:"required,min=1"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}

type PaginatedProfiles struct {
	Data     []*UserPublicProfile `json:"data"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
	HasMore  bool                 `json:"has_more"`
}

type UserInternalResponse struct {
	ID        uint64 `json:"id"`
	Username  string `json:"username"`
	Email     string `json:"email"`
	IsPrivate bool   `json:"is_private"`
	IsActive  bool   `json:"is_active"`
	Role      string `json:"role"`
}
