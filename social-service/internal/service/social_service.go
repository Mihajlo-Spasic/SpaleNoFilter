package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/instagram-clone/social-service/internal/config"
	"github.com/instagram-clone/social-service/internal/models"
	"github.com/instagram-clone/social-service/internal/repository"
)

type SocialService interface {
	// Follow
	SendFollowRequest(followerID, targetID uint64) error
	RespondToFollowRequest(ownerID, followerID uint64, accept bool) error
	Unfollow(followerID, targetID uint64) error
	RemoveFollower(ownerID, followerID uint64) error
	GetFollowers(userID uint64, page, size int) (*models.PaginatedFollowList, error)
	GetFollowing(userID uint64, page, size int) (*models.PaginatedFollowList, error)
	GetPendingRequests(userID uint64, page, size int) (*models.PaginatedFollowList, error)

	// Block
	Block(blockerID, targetID uint64) error
	Unblock(blockerID, targetID uint64) error
	GetBlocked(userID uint64, page, size int) (*models.PaginatedFollowList, error)

	GetStatus(viewerID, targetID uint64) (*models.SocialStatusResponse, error)
	CanView(viewerID, ownerID uint64) (bool, error)
	GetFollowingIDs(userID uint64) ([]uint64, error)
}

type socialService struct {
	followRepo repository.FollowRepository
	blockRepo  repository.BlockRepository
	userClient *userClient
}

func NewSocialService(fr repository.FollowRepository, br repository.BlockRepository, cfg *config.Config) SocialService {
	return &socialService{
		followRepo: fr,
		blockRepo:  br,
		userClient: newUserClient(cfg),
	}
}

func (s *socialService) SendFollowRequest(followerID, targetID uint64) error {
	if followerID == targetID {
		return fmt.Errorf("cannot follow yourself")
	}

	// Check if blocked in either direction
	blocked, err := s.blockRepo.EitherBlocked(followerID, targetID)
	if err != nil {
		return err
	}
	if blocked {
		return fmt.Errorf("cannot follow this user")
	}

	// Fetch target profile to check if private
	isPrivate, err := s.userClient.IsPrivate(targetID)
	if err != nil {
		return fmt.Errorf("could not check target profile: %w", err)
	}

	status := models.FollowStatusAccepted
	if isPrivate {
		status = models.FollowStatusPending
	}

	return s.followRepo.Upsert(followerID, targetID, status)
}

func (s *socialService) RespondToFollowRequest(ownerID, followerID uint64, accept bool) error {
	f, err := s.followRepo.Find(followerID, ownerID)
	if err != nil {
		return err
	}
	if f == nil || f.Status != models.FollowStatusPending {
		return fmt.Errorf("no pending request found")
	}

	if accept {
		return s.followRepo.UpdateStatus(followerID, ownerID, models.FollowStatusAccepted)
	}
	// Reject = delete
	return s.followRepo.Delete(followerID, ownerID)
}

func (s *socialService) Unfollow(followerID, targetID uint64) error {
	return s.followRepo.Delete(followerID, targetID)
}

func (s *socialService) RemoveFollower(ownerID, followerID uint64) error {
	return s.followRepo.RemoveFollower(ownerID, followerID)
}

func (s *socialService) GetFollowers(userID uint64, page, size int) (*models.PaginatedFollowList, error) {
	norm(&page, &size)
	entries, total, err := s.followRepo.ListFollowers(userID, page, size)
	if err != nil {
		return nil, err
	}
	return paginate(entries, total, page, size), nil
}

func (s *socialService) GetFollowing(userID uint64, page, size int) (*models.PaginatedFollowList, error) {
	norm(&page, &size)
	entries, total, err := s.followRepo.ListFollowing(userID, page, size)
	if err != nil {
		return nil, err
	}
	return paginate(entries, total, page, size), nil
}

func (s *socialService) GetPendingRequests(userID uint64, page, size int) (*models.PaginatedFollowList, error) {
	norm(&page, &size)
	entries, total, err := s.followRepo.ListPendingRequests(userID, page, size)
	if err != nil {
		return nil, err
	}
	return paginate(entries, total, page, size), nil
}

func (s *socialService) Block(blockerID, targetID uint64) error {
	if blockerID == targetID {
		return fmt.Errorf("cannot block yourself")
	}
	// Remove any follow relationship in both directions
	s.followRepo.Delete(blockerID, targetID)
	s.followRepo.Delete(targetID, blockerID)
	return s.blockRepo.Block(blockerID, targetID)
}

func (s *socialService) Unblock(blockerID, targetID uint64) error {
	return s.blockRepo.Unblock(blockerID, targetID)
}

func (s *socialService) GetBlocked(userID uint64, page, size int) (*models.PaginatedFollowList, error) {
	norm(&page, &size)
	entries, total, err := s.blockRepo.ListBlocked(userID, page, size)
	if err != nil {
		return nil, err
	}
	return paginate(entries, total, page, size), nil
}

func (s *socialService) GetStatus(viewerID, targetID uint64) (*models.SocialStatusResponse, error) {
	resp := &models.SocialStatusResponse{}

	isFollowing, err := s.followRepo.IsFollowing(viewerID, targetID)
	if err != nil {
		return nil, err
	}
	resp.IsFollowing = isFollowing

	f, err := s.followRepo.Find(viewerID, targetID)
	if err != nil {
		return nil, err
	}
	if f != nil {
		resp.FollowStatus = string(f.Status)
	}

	isFollowedBy, _ := s.followRepo.IsFollowing(targetID, viewerID)
	resp.IsFollowedBy = isFollowedBy

	isBlocking, _ := s.blockRepo.IsBlocked(viewerID, targetID)
	resp.IsBlocking = isBlocking

	isBlockedBy, _ := s.blockRepo.IsBlocked(targetID, viewerID)
	resp.IsBlockedBy = isBlockedBy

	return resp, nil
}

func (s *socialService) CanView(viewerID, ownerID uint64) (bool, error) {
	if viewerID == ownerID {
		return true, nil
	}

	// Check block
	blocked, err := s.blockRepo.EitherBlocked(viewerID, ownerID)
	if err != nil {
		return false, err
	}
	if blocked {
		return false, nil
	}

	// Check if owner is private
	isPrivate, err := s.userClient.IsPrivate(ownerID)
	if err != nil {
		return false, err
	}
	if !isPrivate {
		return true, nil
	}

	// Private profile: viewer must be an accepted follower
	isFollowing, err := s.followRepo.IsFollowing(viewerID, ownerID)
	return isFollowing, err
}

func (s *socialService) GetFollowingIDs(userID uint64) ([]uint64, error) {
	return s.followRepo.GetFollowingIDs(userID)
}

type userClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func newUserClient(cfg *config.Config) *userClient {
	return &userClient{
		baseURL:    cfg.UserServiceURL,
		secret:     cfg.InternalSecret,
		httpClient: &http.Client{Timeout: 4 * time.Second},
	}
}

func (c *userClient) IsPrivate(userID uint64) (bool, error) {
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/internal/users/%d/is-private", c.baseURL, userID), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("X-Internal-Secret", c.secret)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var body struct {
		IsPrivate bool `json:"is_private"`
	}
	json.NewDecoder(resp.Body).Decode(&body)
	return body.IsPrivate, nil
}

func norm(page, size *int) {
	if *page <= 0 {
		*page = 1
	}
	if *size <= 0 || *size > 50 {
		*size = 20
	}
}

func paginate(data []models.FollowEntry, total int64, page, size int) *models.PaginatedFollowList {
	return &models.PaginatedFollowList{
		Data:     data,
		Total:    total,
		Page:     page,
		PageSize: size,
		HasMore:  int64(page*size) < total,
	}
}

// keep compiler happy
var _ = bytes.NewBuffer
var _ = json.Marshal
var _ = fmt.Sprintf
