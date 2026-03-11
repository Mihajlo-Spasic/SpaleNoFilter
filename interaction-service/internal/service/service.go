package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/instagram-clone/interaction-service/internal/config"
	"github.com/instagram-clone/interaction-service/internal/models"
	"github.com/instagram-clone/interaction-service/internal/repository"
)

type InteractionService interface {
	Like(userID, postID uint64) error
	Unlike(userID, postID uint64) error
	HasLiked(userID, postID uint64) (bool, error)

	AddComment(userID, postID uint64, body string) (*models.Comment, error)
	UpdateComment(commentID, userID uint64, body string) error
	DeleteComment(commentID, userID uint64) error
	ListComments(postID, viewerID uint64, page, size int) (*models.PaginatedComments, error)
}

type interactionService struct {
	likeRepo    repository.LikeRepository
	commentRepo repository.CommentRepository
	social      *socialClient
	post        *postClient
}

func NewInteractionService(lr repository.LikeRepository, cr repository.CommentRepository, cfg *config.Config) InteractionService {
	return &interactionService{
		likeRepo:    lr,
		commentRepo: cr,
		social:      newSocialClient(cfg),
		post:        newPostClient(cfg),
	}
}

func (s *interactionService) Like(userID, postID uint64) error {
	// Verify access to the post's owner
	postOwner, err := s.post.GetPostOwner(postID)
	if err != nil {
		return fmt.Errorf("post not found")
	}

	if userID != postOwner {
		canView, _ := s.social.CanView(userID, postOwner)
		if !canView {
			return fmt.Errorf("access denied")
		}
	}

	added, err := s.likeRepo.Like(userID, postID)
	if err != nil {
		return err
	}
	if added {
		s.post.IncrementLike(postID, 1)
	}
	return nil
}

func (s *interactionService) Unlike(userID, postID uint64) error {
	removed, err := s.likeRepo.Unlike(userID, postID)
	if err != nil {
		return err
	}
	if removed {
		s.post.IncrementLike(postID, -1)
	}
	return nil
}

func (s *interactionService) HasLiked(userID, postID uint64) (bool, error) {
	return s.likeRepo.HasLiked(userID, postID)
}

func (s *interactionService) AddComment(userID, postID uint64, body string) (*models.Comment, error) {
	postOwner, err := s.post.GetPostOwner(postID)
	if err != nil {
		return nil, fmt.Errorf("post not found")
	}

	if userID != postOwner {
		canView, _ := s.social.CanView(userID, postOwner)
		if !canView {
			return nil, fmt.Errorf("access denied")
		}
	}

	comment, err := s.commentRepo.Create(&models.Comment{PostID: postID, UserID: userID, Body: body})
	if err != nil {
		return nil, err
	}

	s.post.IncrementComment(postID, 1)
	return comment, nil
}

func (s *interactionService) UpdateComment(commentID, userID uint64, body string) error {
	c, err := s.commentRepo.FindByID(commentID)
	if err != nil || c == nil {
		return fmt.Errorf("comment not found")
	}
	if c.UserID != userID {
		return fmt.Errorf("not authorized")
	}
	return s.commentRepo.Update(commentID, userID, body)
}

func (s *interactionService) DeleteComment(commentID, userID uint64) error {
	c, err := s.commentRepo.FindByID(commentID)
	if err != nil || c == nil {
		return fmt.Errorf("comment not found")
	}
	if c.UserID != userID {
		return fmt.Errorf("not authorized")
	}
	if err := s.commentRepo.SoftDelete(commentID, userID); err != nil {
		return err
	}
	s.post.IncrementComment(c.PostID, -1)
	return nil
}

func (s *interactionService) ListComments(postID, viewerID uint64, page, size int) (*models.PaginatedComments, error) {
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 50 {
		size = 20
	}

	postOwner, err := s.post.GetPostOwner(postID)
	if err != nil {
		return nil, fmt.Errorf("post not found")
	}
	if viewerID != postOwner {
		canView, _ := s.social.CanView(viewerID, postOwner)
		if !canView {
			return nil, fmt.Errorf("access denied")
		}
	}

	comments, total, err := s.commentRepo.ListByPost(postID, page, size)
	if err != nil {
		return nil, err
	}
	return &models.PaginatedComments{
		Data:     comments,
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
	return &socialClient{cfg.SocialServiceURL, cfg.InternalSecret, &http.Client{Timeout: 4 * time.Second}}
}

func (c *socialClient) CanView(viewerID, ownerID uint64) (bool, error) {
	body, _ := json.Marshal(map[string]uint64{"viewer_id": viewerID, "owner_id": ownerID})
	req, _ := http.NewRequest(http.MethodPost, c.baseURL+"/internal/social/can-view", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", c.secret)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return true, nil
	}
	defer resp.Body.Close()
	var result struct {
		CanView bool `json:"can_view"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.CanView, nil
}

type postClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func newPostClient(cfg *config.Config) *postClient {
	return &postClient{cfg.PostServiceURL, cfg.InternalSecret, &http.Client{Timeout: 4 * time.Second}}
}

func (c *postClient) GetPostOwner(postID uint64) (uint64, error) {
	req, _ := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/internal/posts/%d/owner", c.baseURL, postID),
		nil,
	)
	req.Header.Set("X-Internal-Secret", c.secret)
	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("post not found")
	}
	defer resp.Body.Close()
	var result struct {
		UserID uint64 `json:"user_id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.UserID, nil
}

func (c *postClient) IncrementLike(postID uint64, delta int) {
	body, _ := json.Marshal(map[string]int{"delta": delta})
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/internal/posts/%d/like-count", c.baseURL, postID),
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", c.secret)
	c.httpClient.Do(req)
}

func (c *postClient) IncrementComment(postID uint64, delta int) {
	body, _ := json.Marshal(map[string]int{"delta": delta})
	req, _ := http.NewRequest(http.MethodPost,
		fmt.Sprintf("%s/internal/posts/%d/comment-count", c.baseURL, postID),
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", c.secret)
	c.httpClient.Do(req)
}
