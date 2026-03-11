package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/instagram-clone/post-service/internal/config"
	"github.com/instagram-clone/post-service/internal/models"
	"github.com/instagram-clone/post-service/internal/repository"
)

type PostService interface {
	CreatePost(userID uint64, caption string, files []*multipart.FileHeader) (*models.Post, error)
	GetPost(postID, viewerID uint64) (*models.Post, error)
	GetPostOwnerInternal(postID uint64) (uint64, error)
	UpdateCaption(postID, userID uint64, caption string) error
	DeleteMedia(postID, mediaID, userID uint64) error
	DeletePost(postID, userID uint64) error
	GetUserPosts(ownerID, viewerID uint64, page, size int) (*models.PaginatedSummaries, error)
	GetPostsByUserIDs(userIDs []uint64, page, size int) (*models.PaginatedPosts, error)
	IncrementLike(postID uint64, delta int) error
	IncrementComment(postID uint64, delta int) error
	// UploadFile uploads a single file to storage and returns its public URL.

	UploadFile(userID uint64, file multipart.File, header *multipart.FileHeader) (string, error)
}

type postService struct {
	repo         repository.PostRepository
	storage      MediaStorage
	socialClient *socialClient
	cfg          *config.Config
}

func NewPostService(repo repository.PostRepository, storage MediaStorage, cfg *config.Config) PostService {
	return &postService{
		repo:         repo,
		storage:      storage,
		socialClient: newSocialClient(cfg),
		cfg:          cfg,
	}
}

func (s *postService) CreatePost(userID uint64, caption string, files []*multipart.FileHeader) (*models.Post, error) {
	if len(files) == 0 {
		return nil, fmt.Errorf("at least one media file is required")
	}
	if len(files) > s.cfg.MaxMediaPerPost {
		return nil, fmt.Errorf("maximum %d media items per post", s.cfg.MaxMediaPerPost)
	}

	post, err := s.repo.Create(&models.Post{UserID: userID, Caption: caption})
	if err != nil {
		return nil, fmt.Errorf("create post record: %w", err)
	}

	for i, fh := range files {
		f, err := fh.Open()
		if err != nil {
			return nil, fmt.Errorf("open file %d: %w", i, err)
		}
		defer f.Close()

		media, err := s.storage.Upload(f, fh, userID)
		if err != nil {
			s.repo.SoftDelete(post.ID)
			return nil, err
		}
		media.PostID = post.ID
		media.Position = i

		if err := s.repo.AddMedia(media); err != nil {
			return nil, fmt.Errorf("save media record: %w", err)
		}
	}

	return s.repo.FindByID(post.ID)
}

func (s *postService) GetPost(postID, viewerID uint64) (*models.Post, error) {
	post, err := s.repo.FindByID(postID)
	if err != nil || post == nil {
		return nil, fmt.Errorf("post not found")
	}

	if viewerID != post.UserID {
		canView, err := s.socialClient.CanView(viewerID, post.UserID)
		if err != nil || !canView {
			return nil, fmt.Errorf("access denied")
		}
	}

	return post, nil
}

func (s *postService) UpdateCaption(postID, userID uint64, caption string) error {
	post, err := s.repo.FindByID(postID)
	if err != nil || post == nil {
		return fmt.Errorf("post not found")
	}
	if post.UserID != userID {
		return fmt.Errorf("not authorized")
	}
	return s.repo.UpdateCaption(postID, caption)
}

func (s *postService) DeleteMedia(postID, mediaID, userID uint64) error {
	post, err := s.repo.FindByID(postID)
	if err != nil || post == nil {
		return fmt.Errorf("post not found")
	}
	if post.UserID != userID {
		return fmt.Errorf("not authorized")
	}

	media, err := s.repo.FindMediaByID(mediaID)
	if err != nil || media == nil {
		return fmt.Errorf("media not found")
	}

	if err := s.repo.DeleteMedia(mediaID, postID); err != nil {
		return err
	}

	s.storage.Delete(media.MediaKey)
	return nil
}

func (s *postService) DeletePost(postID, userID uint64) error {
	post, err := s.repo.FindByID(postID)
	if err != nil || post == nil {
		return fmt.Errorf("post not found")
	}
	if post.UserID != userID {
		return fmt.Errorf("not authorized")
	}

	// Delete all media from storage
	for _, m := range post.Media {
		s.storage.Delete(m.MediaKey)
	}
	return s.repo.SoftDelete(postID)
}

func (s *postService) GetUserPosts(ownerID, viewerID uint64, page, size int) (*models.PaginatedSummaries, error) {
	norm(&page, &size)

	if viewerID != ownerID {
		canView, err := s.socialClient.CanView(viewerID, ownerID)
		if err != nil || !canView {
			return nil, fmt.Errorf("access denied")
		}
	}

	summaries, total, err := s.repo.ListByUser(ownerID, page, size)
	if err != nil {
		return nil, err
	}
	return &models.PaginatedSummaries{
		Data:     summaries,
		Total:    total,
		Page:     page,
		PageSize: size,
		HasMore:  int64(page*size) < total,
	}, nil
}

func (s *postService) GetPostsByUserIDs(userIDs []uint64, page, size int) (*models.PaginatedPosts, error) {
	norm(&page, &size)
	posts, total, err := s.repo.ListByUserIDs(userIDs, page, size)
	if err != nil {
		return nil, err
	}
	return &models.PaginatedPosts{
		Data:     posts,
		Total:    total,
		Page:     page,
		PageSize: size,
		HasMore:  int64(page*size) < total,
	}, nil
}

func (s *postService) GetPostOwnerInternal(postID uint64) (uint64, error) {
	post, err := s.repo.FindByID(postID)
	if err != nil || post == nil {
		return 0, fmt.Errorf("post not found")
	}
	return post.UserID, nil
}

func (s *postService) IncrementLike(postID uint64, delta int) error {
	return s.repo.IncrementLike(postID, delta)
}

func (s *postService) IncrementComment(postID uint64, delta int) error {
	return s.repo.IncrementComment(postID, delta)
}

func (s *postService) UploadFile(userID uint64, file multipart.File, header *multipart.FileHeader) (string, error) {
	media, err := s.storage.Upload(file, header, userID)
	if err != nil {
		return "", err
	}
	return media.MediaURL, nil
}

type socialClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func newSocialClient(cfg *config.Config) *socialClient {
	return &socialClient{
		baseURL:    cfg.SocialServiceURL,
		secret:     cfg.InternalSecret,
		httpClient: &http.Client{Timeout: 4 * time.Second},
	}
}

func (c *socialClient) CanView(viewerID, ownerID uint64) (bool, error) {
	body, _ := json.Marshal(map[string]uint64{"viewer_id": viewerID, "owner_id": ownerID})
	req, _ := http.NewRequest(http.MethodPost, c.baseURL+"/internal/social/can-view", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Secret", c.secret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		// Default allow if social service is unavailable (best-effort)
		return true, nil
	}
	defer resp.Body.Close()

	var result struct {
		CanView bool `json:"can_view"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	return result.CanView, nil
}

func norm(page, size *int) {
	if *page <= 0 {
		*page = 1
	}
	if *size <= 0 || *size > 50 {
		*size = 20
	}
}
