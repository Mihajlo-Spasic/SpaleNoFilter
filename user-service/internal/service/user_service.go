package service

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/instagram-clone/user-service/internal/config"
	"github.com/instagram-clone/user-service/internal/models"
	"github.com/instagram-clone/user-service/internal/repository"
)

// UserService owns: auth flow, profile CRUD, search.
// Social graph (follow/block) is entirely in social-service.
type UserService interface {
	// Auth
	Register(req *models.RegisterRequest, userAgent, ipAddress string) (*models.AuthResponse, error)
	Login(req *models.LoginRequest, userAgent, ipAddress string) (*models.AuthResponse, error)
	Logout(userID uint64, token string) error
	LogoutAll(userID uint64) error
	RefreshToken(refreshToken string) (*models.AuthResponse, error)
	ChangePassword(userID uint64, req *models.ChangePasswordRequest) error

	// Profile
	GetMyProfile(userID uint64) (*models.User, error)
	GetPublicProfile(username string) (*models.UserPublicProfile, error)
	GetPublicProfileByID(id uint64) (*models.UserPublicProfile, error)
	UpdateProfile(userID uint64, req *models.UpdateProfileRequest) (*models.User, error)
	UpdateAvatar(userID uint64, avatarURL string) error
	DeleteAccount(userID uint64) error

	// Search
	SearchUsers(query string, page, pageSize int) (*models.PaginatedProfiles, error)

	GetUserByID(id uint64) (*models.UserInternalResponse, error)
	GetUserByUsername(username string) (*models.UserInternalResponse, error)
}

type userService struct {
	userRepo   repository.UserRepository
	authClient AuthClient
	cfg        *config.Config
}

func NewUserService(
	userRepo repository.UserRepository,
	authClient AuthClient,
	cfg *config.Config,
) UserService {
	return &userService{
		userRepo:   userRepo,
		authClient: authClient,
		cfg:        cfg,
	}
}

func (s *userService) Register(req *models.RegisterRequest, userAgent, ipAddress string) (*models.AuthResponse, error) {
	if exists, _ := s.userRepo.ExistsByUsername(req.Username); exists {
		return nil, fmt.Errorf("username already taken")
	}
	if exists, _ := s.userRepo.ExistsByEmail(req.Email); exists {
		return nil, fmt.Errorf("email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.userRepo.Create(&models.User{
		Username:     req.Username,
		Email:        req.Email,
		PasswordHash: string(hash),
		FullName:     req.FullName,
	})
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	pair, err := s.authClient.IssueTokens(&IssueTokenRequest{
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		UserAgent: userAgent,
		IPAddress: ipAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("issue tokens: %w", err)
	}

	return &models.AuthResponse{
		User:         toPublicProfile(user),
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    pair.ExpiresAt,
		TokenType:    pair.TokenType,
	}, nil
}

func (s *userService) Login(req *models.LoginRequest, userAgent, ipAddress string) (*models.AuthResponse, error) {
	user, err := s.userRepo.FindByUsernameOrEmail(req.UsernameOrEmail)
	if err != nil || user == nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if !user.IsActive {
		return nil, fmt.Errorf("account is deactivated")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	pair, err := s.authClient.IssueTokens(&IssueTokenRequest{
		UserID:    user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Role:      user.Role,
		UserAgent: userAgent,
		IPAddress: ipAddress,
	})
	if err != nil {
		return nil, fmt.Errorf("issue tokens: %w", err)
	}

	return &models.AuthResponse{
		User:         toPublicProfile(user),
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    pair.ExpiresAt,
		TokenType:    pair.TokenType,
	}, nil
}

// Logout revokes the current access token via auth-service.
func (s *userService) Logout(userID uint64, token string) error {
	return s.authClient.RevokeToken(token)
}

func (s *userService) LogoutAll(userID uint64) error {
	return s.authClient.InvalidateUserTokens(userID)
}

func (s *userService) RefreshToken(refreshToken string) (*models.AuthResponse, error) {
	pair, err := s.authClient.RefreshTokens(refreshToken)
	if err != nil {
		return nil, err
	}
	return &models.AuthResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		ExpiresAt:    pair.ExpiresAt,
		TokenType:    pair.TokenType,
	}, nil
}

func (s *userService) ChangePassword(userID uint64, req *models.ChangePasswordRequest) error {
	user, err := s.userRepo.FindByID(userID)
	if err != nil || user == nil {
		return fmt.Errorf("user not found")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.OldPassword)); err != nil {
		return fmt.Errorf("old password is incorrect")
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.userRepo.UpdatePassword(userID, string(newHash)); err != nil {
		return err
	}
	// Invalidate ALL sessions after a password change
	return s.authClient.InvalidateUserTokens(userID)
}

func (s *userService) GetMyProfile(userID uint64) (*models.User, error) {
	return s.userRepo.FindByID(userID)
}

func (s *userService) GetPublicProfile(username string) (*models.UserPublicProfile, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return toPublicProfile(user), nil
}

func (s *userService) GetPublicProfileByID(id uint64) (*models.UserPublicProfile, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil || user == nil {
		return nil, fmt.Errorf("user not found")
	}
	return toPublicProfile(user), nil
}

func (s *userService) UpdateProfile(userID uint64, req *models.UpdateProfileRequest) (*models.User, error) {
	updates := map[string]interface{}{}
	if req.FullName != nil {
		updates["full_name"] = *req.FullName
	}
	if req.Bio != nil {
		updates["bio"] = *req.Bio
	}
	if req.Website != nil {
		updates["website"] = *req.Website
	}
	if req.IsPrivate != nil {
		updates["is_private"] = *req.IsPrivate
	}
	if err := s.userRepo.Update(userID, updates); err != nil {
		return nil, err
	}
	return s.userRepo.FindByID(userID)
}

func (s *userService) UpdateAvatar(userID uint64, avatarURL string) error {
	return s.userRepo.Update(userID, map[string]interface{}{"avatar_url": avatarURL})
}

func (s *userService) DeleteAccount(userID uint64) error {
	// Revoke all sessions first
	if err := s.authClient.InvalidateUserTokens(userID); err != nil {
		return err
	}
	return s.userRepo.SoftDelete(userID)
}

func (s *userService) SearchUsers(query string, page, pageSize int) (*models.PaginatedProfiles, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 50 {
		pageSize = 20
	}
	users, total, err := s.userRepo.Search(query, page, pageSize)
	if err != nil {
		return nil, err
	}
	profiles := make([]*models.UserPublicProfile, len(users))
	for i, u := range users {
		profiles[i] = toPublicProfile(u)
	}
	return &models.PaginatedProfiles{
		Data:     profiles,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		HasMore:  int64(page*pageSize) < total,
	}, nil
}

func (s *userService) GetUserByID(id uint64) (*models.UserInternalResponse, error) {
	u, err := s.userRepo.FindByID(id)
	if err != nil || u == nil {
		return nil, fmt.Errorf("user not found")
	}
	return toInternal(u), nil
}

func (s *userService) GetUserByUsername(username string) (*models.UserInternalResponse, error) {
	u, err := s.userRepo.FindByUsername(username)
	if err != nil || u == nil {
		return nil, fmt.Errorf("user not found")
	}
	return toInternal(u), nil
}

func toPublicProfile(u *models.User) *models.UserPublicProfile {
	return &models.UserPublicProfile{
		ID:             u.ID,
		Username:       u.Username,
		FullName:       u.FullName,
		Bio:            u.Bio,
		AvatarURL:      u.AvatarURL,
		Website:        u.Website,
		IsPrivate:      u.IsPrivate,
		IsVerified:     u.IsVerified,
		PostCount:      u.PostCount,
		FollowerCount:  u.FollowerCount,
		FollowingCount: u.FollowingCount,
		CreatedAt:      u.CreatedAt,
	}
}

func toInternal(u *models.User) *models.UserInternalResponse {
	return &models.UserInternalResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		IsPrivate: u.IsPrivate,
		IsActive:  u.IsActive,
		Role:      u.Role,
	}
}
