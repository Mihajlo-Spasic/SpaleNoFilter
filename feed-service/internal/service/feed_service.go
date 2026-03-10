package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/instagram-clone/feed-service/internal/config"
)

// FeedPost mirrors the post model returned by post-service
type FeedPost struct {
	ID           uint64      `json:"id"`
	UserID       uint64      `json:"user_id"`
	Caption      string      `json:"caption"`
	LikeCount    uint32      `json:"like_count"`
	CommentCount uint32      `json:"comment_count"`
	Media        interface{} `json:"media"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
}

type FeedResponse struct {
	Posts    []*FeedPost `json:"posts"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	HasMore  bool        `json:"has_more"`
}

type FeedService interface {
	GetFeed(userID uint64, page, size int) (*FeedResponse, error)
}

type feedService struct {
	social *socialClient
	post   *postClient
}

func NewFeedService(cfg *config.Config) FeedService {
	return &feedService{
		social: newSocialClient(cfg),
		post:   newPostClient(cfg),
	}
}

func (s *feedService) GetFeed(userID uint64, page, size int) (*FeedResponse, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 50 {
		size = 20
	}

	// 1. Get IDs of people this user follows
	followingIDs, err := s.social.GetFollowingIDs(userID)
	if err != nil {
		return nil, fmt.Errorf("get following ids: %w", err)
	}

	// Include the user's own posts in the feed
	followingIDs = append(followingIDs, userID)

	if len(followingIDs) == 0 {
		return &FeedResponse{Posts: []*FeedPost{}, Page: page, PageSize: size}, nil
	}

	// 2. Fetch posts from those users, chronologically
	posts, total, err := s.post.GetPostsByUserIDs(followingIDs, page, size)
	if err != nil {
		return nil, fmt.Errorf("get posts: %w", err)
	}

	return &FeedResponse{
		Posts:    posts,
		Total:    total,
		Page:     page,
		PageSize: size,
		HasMore:  int64(page*size) < total,
	}, nil
}

type socialClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func newSocialClient(cfg *config.Config) *socialClient {
	return &socialClient{cfg.SocialServiceURL, cfg.InternalSecret, &http.Client{Timeout: 5 * time.Second}}
}

func (c *socialClient) GetFollowingIDs(userID uint64) ([]uint64, error) {
	req, _ := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/internal/social/following-ids/%d", c.baseURL, userID),
		nil,
	)
	req.Header.Set("X-Internal-Secret", c.secret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		FollowingIDs []uint64 `json:"following_ids"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.FollowingIDs, nil
}

type postClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func newPostClient(cfg *config.Config) *postClient {
	return &postClient{cfg.PostServiceURL, cfg.InternalSecret, &http.Client{Timeout: 5 * time.Second}}
}

func (c *postClient) GetPostsByUserIDs(userIDs []uint64, page, size int) ([]*FeedPost, int64, error) {
	body, _ := json.Marshal(map[string]interface{}{
		"user_ids":  userIDs,
		"page":      page,
		"page_size": size,
	})

	req, _ := http.NewRequest(http.MethodPost,
		c.baseURL+"/internal/posts/by-user-ids",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", c.secret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Data  []*FeedPost `json:"data"`
		Total int64       `json:"total"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.Data, result.Total, nil
}
