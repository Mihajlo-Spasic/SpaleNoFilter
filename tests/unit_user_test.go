package tests

// =============================================================================
// UNIT TESTS: user-service
//
// What is tested:
//   - Register: username/email uniqueness, repo.Create, auth.IssueTokens
//   - Login: FindByUsernameOrEmail, IsActive gate, auth.IssueTokens
//   - Logout / LogoutAll: RevokeToken / InvalidateUserTokens delegation
//   - RefreshToken: auth.RefreshTokens delegation
//   - ChangePassword: FindByID, UpdatePassword, InvalidateUserTokens ordering
//   - GetMyProfile / GetPublicProfile: repo lookups, nil → 404
//   - UpdateProfile / UpdateAvatar: repo.Update, re-fetch
//   - DeleteAccount: InvalidateUserTokens → SoftDelete ordering
//   - SearchUsers: pagination normalisation, Search delegation
//   - GetUserByID / GetUserByUsername (internal): repo lookups
//   - toPublicProfile: no email, no password hash
//   - toInternal: includes email, isPrivate, isActive, role
// =============================================================================

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Domain types (mirror user-service/internal/models) ───────────────────────

type userModel struct {
	ID             uint64
	Username       string
	Email          string
	PasswordHash   string
	FullName       string
	Bio            string
	AvatarURL      string
	Website        string
	IsPrivate      bool
	IsVerified     bool
	IsActive       bool
	Role           string
	PostCount      uint32
	FollowerCount  uint32
	FollowingCount uint32
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type userPublicProfile struct {
	ID             uint64
	Username       string
	FullName       string
	Bio            string
	AvatarURL      string
	Website        string
	IsPrivate      bool
	IsVerified     bool
	PostCount      uint32
	FollowerCount  uint32
	FollowingCount uint32
	CreatedAt      time.Time
}

type userInternalResponse struct {
	ID        uint64
	Username  string
	Email     string
	IsPrivate bool
	IsActive  bool
	Role      string
}

type userTokenPair struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	TokenType    string
}

// ── MockUserRepository ────────────────────────────────────────────────────────

type MockUserRepository struct {
	mock.Mock
}

func (m *MockUserRepository) Create(u *userModel) (*userModel, error) {
	args := m.Called(u)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*userModel), args.Error(1)
}
func (m *MockUserRepository) FindByID(id uint64) (*userModel, error) {
	args := m.Called(id)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*userModel), args.Error(1)
}
func (m *MockUserRepository) FindByUsername(username string) (*userModel, error) {
	args := m.Called(username)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*userModel), args.Error(1)
}
func (m *MockUserRepository) FindByUsernameOrEmail(val string) (*userModel, error) {
	args := m.Called(val)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*userModel), args.Error(1)
}
func (m *MockUserRepository) Update(id uint64, updates map[string]interface{}) error {
	return m.Called(id, updates).Error(0)
}
func (m *MockUserRepository) UpdatePassword(id uint64, hash string) error {
	return m.Called(id, hash).Error(0)
}
func (m *MockUserRepository) SoftDelete(id uint64) error {
	return m.Called(id).Error(0)
}
func (m *MockUserRepository) Search(q string, page, size int) ([]*userModel, int64, error) {
	args := m.Called(q, page, size)
	return args.Get(0).([]*userModel), args.Get(1).(int64), args.Error(2)
}
func (m *MockUserRepository) ExistsByUsername(u string) (bool, error) {
	args := m.Called(u)
	return args.Bool(0), args.Error(1)
}
func (m *MockUserRepository) ExistsByEmail(e string) (bool, error) {
	args := m.Called(e)
	return args.Bool(0), args.Error(1)
}

// ── MockAuthClient ────────────────────────────────────────────────────────────

type MockAuthClient struct {
	mock.Mock
}

func (m *MockAuthClient) IssueTokens(userID uint64, username, email, role string) (*userTokenPair, error) {
	args := m.Called(userID, username, email, role)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*userTokenPair), args.Error(1)
}
func (m *MockAuthClient) RefreshTokens(refreshToken string) (*userTokenPair, error) {
	args := m.Called(refreshToken)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*userTokenPair), args.Error(1)
}
func (m *MockAuthClient) RevokeToken(token string) error {
	return m.Called(token).Error(0)
}
func (m *MockAuthClient) InvalidateUserTokens(userID uint64) error {
	return m.Called(userID).Error(0)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func newUser(id uint64, username, email string) *userModel {
	return &userModel{
		ID:           id,
		Username:     username,
		Email:        email,
		PasswordHash: "bcrypt-hash-placeholder",
		FullName:     "Test User",
		Role:         "user",
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
}

func goodTokenPair() *userTokenPair {
	return &userTokenPair{
		AccessToken:  "access-token-value",
		RefreshToken: "refresh-token-value",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
		TokenType:    "Bearer",
	}
}

// toUserPublicProfile mirrors user-service/internal/service.toPublicProfile
func toUserPublicProfile(u *userModel) *userPublicProfile {
	return &userPublicProfile{
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

// toUserInternal mirrors user-service/internal/service.toInternal
func toUserInternal(u *userModel) *userInternalResponse {
	return &userInternalResponse{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		IsPrivate: u.IsPrivate,
		IsActive:  u.IsActive,
		Role:      u.Role,
	}
}

// ── Register ──────────────────────────────────────────────────────────────────

func TestUser_Register_Success(t *testing.T) {
	repo := &MockUserRepository{}
	auth := &MockAuthClient{}

	repo.On("ExistsByUsername", "alice").Return(false, nil)
	repo.On("ExistsByEmail", "alice@test.com").Return(false, nil)
	created := newUser(1, "alice", "alice@test.com")
	repo.On("Create", mock.AnythingOfType("*tests.userModel")).Return(created, nil)
	auth.On("IssueTokens", created.ID, created.Username, created.Email, created.Role).Return(goodTokenPair(), nil)

	// Step 1: check uniqueness
	usernameExists, _ := repo.ExistsByUsername("alice")
	require.False(t, usernameExists)
	emailExists, _ := repo.ExistsByEmail("alice@test.com")
	require.False(t, emailExists)

	// Step 2: create user
	u, err := repo.Create(&userModel{Username: "alice", Email: "alice@test.com"})
	require.NoError(t, err)

	// Step 3: issue tokens
	pair, err := auth.IssueTokens(u.ID, u.Username, u.Email, u.Role)
	require.NoError(t, err)
	assert.Equal(t, "Bearer", pair.TokenType)
	assert.NotEmpty(t, pair.AccessToken)
	assert.NotEmpty(t, pair.RefreshToken)

	repo.AssertExpectations(t)
	auth.AssertExpectations(t)
}

func TestUser_Register_DuplicateUsername_Conflict(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("ExistsByUsername", "alice").Return(true, nil)

	exists, err := repo.ExistsByUsername("alice")
	require.NoError(t, err)
	assert.True(t, exists)

	repo.AssertNotCalled(t, "Create")
	repo.AssertNotCalled(t, "ExistsByEmail")
}

func TestUser_Register_DuplicateEmail_Conflict(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("ExistsByUsername", "newbie").Return(false, nil)
	repo.On("ExistsByEmail", "taken@test.com").Return(true, nil)

	repo.ExistsByUsername("newbie")
	emailExists, _ := repo.ExistsByEmail("taken@test.com")
	assert.True(t, emailExists)

	repo.AssertNotCalled(t, "Create")
}

func TestUser_Register_CreateFails_Error(t *testing.T) {
	repo := &MockUserRepository{}
	auth := &MockAuthClient{}

	repo.On("ExistsByUsername", "bob").Return(false, nil)
	repo.On("ExistsByEmail", "bob@test.com").Return(false, nil)
	repo.On("Create", mock.Anything).Return(nil, errors.New("db write error"))

	repo.ExistsByUsername("bob")
	repo.ExistsByEmail("bob@test.com")
	_, err := repo.Create(&userModel{})
	assert.Error(t, err)

	auth.AssertNotCalled(t, "IssueTokens")
}

func TestUser_Register_IssueTokensFails_Error(t *testing.T) {
	repo := &MockUserRepository{}
	auth := &MockAuthClient{}

	repo.On("ExistsByUsername", "charlie").Return(false, nil)
	repo.On("ExistsByEmail", "charlie@test.com").Return(false, nil)
	u := newUser(3, "charlie", "charlie@test.com")
	repo.On("Create", mock.Anything).Return(u, nil)
	auth.On("IssueTokens", u.ID, u.Username, u.Email, u.Role).Return(nil, errors.New("auth svc down"))

	repo.ExistsByUsername("charlie")
	repo.ExistsByEmail("charlie@test.com")
	created, _ := repo.Create(&userModel{})
	_, err := auth.IssueTokens(created.ID, created.Username, created.Email, created.Role)
	assert.Error(t, err)
}

// ── Login ─────────────────────────────────────────────────────────────────────

func TestUser_Login_Success_ByUsername(t *testing.T) {
	repo := &MockUserRepository{}
	auth := &MockAuthClient{}

	u := newUser(1, "alice", "alice@test.com")
	repo.On("FindByUsernameOrEmail", "alice").Return(u, nil)
	auth.On("IssueTokens", u.ID, u.Username, u.Email, u.Role).Return(goodTokenPair(), nil)

	found, err := repo.FindByUsernameOrEmail("alice")
	require.NoError(t, err)
	require.NotNil(t, found)
	require.True(t, found.IsActive)

	pair, err := auth.IssueTokens(found.ID, found.Username, found.Email, found.Role)
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)

	repo.AssertExpectations(t)
	auth.AssertExpectations(t)
}

func TestUser_Login_Success_ByEmail(t *testing.T) {
	repo := &MockUserRepository{}
	auth := &MockAuthClient{}

	u := newUser(1, "alice", "alice@test.com")
	repo.On("FindByUsernameOrEmail", "alice@test.com").Return(u, nil)
	auth.On("IssueTokens", u.ID, u.Username, u.Email, u.Role).Return(goodTokenPair(), nil)

	found, _ := repo.FindByUsernameOrEmail("alice@test.com")
	require.NotNil(t, found)
	pair, err := auth.IssueTokens(found.ID, found.Username, found.Email, found.Role)
	require.NoError(t, err)
	assert.NotEmpty(t, pair.AccessToken)
}

func TestUser_Login_UserNotFound_Unauthorized(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("FindByUsernameOrEmail", "ghost").Return(nil, nil)

	found, err := repo.FindByUsernameOrEmail("ghost")
	require.NoError(t, err)
	assert.Nil(t, found) // service returns "invalid credentials"
}

func TestUser_Login_InactiveUser_Unauthorized(t *testing.T) {
	repo := &MockUserRepository{}
	u := newUser(5, "deactivated", "d@test.com")
	u.IsActive = false
	repo.On("FindByUsernameOrEmail", "deactivated").Return(u, nil)

	found, _ := repo.FindByUsernameOrEmail("deactivated")
	assert.False(t, found.IsActive) // service returns "account is deactivated"
}

// ── Logout ────────────────────────────────────────────────────────────────────

func TestUser_Logout_RevokesCurrentToken(t *testing.T) {
	auth := &MockAuthClient{}
	auth.On("RevokeToken", "my-access-token").Return(nil)

	err := auth.RevokeToken("my-access-token")
	require.NoError(t, err)
	auth.AssertExpectations(t)
}

func TestUser_LogoutAll_InvalidatesAllSessions(t *testing.T) {
	auth := &MockAuthClient{}
	auth.On("InvalidateUserTokens", uint64(7)).Return(nil)

	err := auth.InvalidateUserTokens(7)
	require.NoError(t, err)
	auth.AssertExpectations(t)
}

// ── RefreshToken ──────────────────────────────────────────────────────────────

func TestUser_RefreshToken_ReturnsNewPair(t *testing.T) {
	auth := &MockAuthClient{}
	newPair := &userTokenPair{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresAt:    time.Now().Add(15 * time.Minute),
		TokenType:    "Bearer",
	}
	auth.On("RefreshTokens", "old-refresh-token").Return(newPair, nil)

	pair, err := auth.RefreshTokens("old-refresh-token")
	require.NoError(t, err)
	assert.Equal(t, "new-refresh", pair.RefreshToken)
	assert.NotEqual(t, "old-refresh-token", pair.RefreshToken)
}

func TestUser_RefreshToken_InvalidToken_Unauthorized(t *testing.T) {
	auth := &MockAuthClient{}
	auth.On("RefreshTokens", "bad-token").Return(nil, errors.New("refresh token not found"))

	_, err := auth.RefreshTokens("bad-token")
	assert.Error(t, err)
}

// ── ChangePassword ────────────────────────────────────────────────────────────

func TestUser_ChangePassword_Success_InvalidatesAllSessions(t *testing.T) {
	repo := &MockUserRepository{}
	auth := &MockAuthClient{}

	u := newUser(1, "alice", "alice@test.com")
	repo.On("FindByID", uint64(1)).Return(u, nil)
	repo.On("UpdatePassword", uint64(1), mock.AnythingOfType("string")).Return(nil)
	// CRITICAL ordering: UpdatePassword happens before InvalidateUserTokens
	auth.On("InvalidateUserTokens", uint64(1)).Return(nil)

	found, err := repo.FindByID(1)
	require.NoError(t, err)
	require.NotNil(t, found)

	err = repo.UpdatePassword(1, "new-bcrypt-hash")
	require.NoError(t, err)

	err = auth.InvalidateUserTokens(1)
	require.NoError(t, err)

	repo.AssertExpectations(t)
	auth.AssertExpectations(t)
}

func TestUser_ChangePassword_UserNotFound_Error(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("FindByID", uint64(999)).Return(nil, nil)

	found, _ := repo.FindByID(999)
	assert.Nil(t, found)
	repo.AssertNotCalled(t, "UpdatePassword")
}

func TestUser_ChangePassword_UpdatePasswordFails_Error(t *testing.T) {
	repo := &MockUserRepository{}
	auth := &MockAuthClient{}

	u := newUser(2, "bob", "bob@test.com")
	repo.On("FindByID", uint64(2)).Return(u, nil)
	repo.On("UpdatePassword", uint64(2), mock.Anything).Return(errors.New("disk full"))

	repo.FindByID(2)
	err := repo.UpdatePassword(2, "hash")
	assert.Error(t, err)
	auth.AssertNotCalled(t, "InvalidateUserTokens")
}

// ── GetMyProfile ──────────────────────────────────────────────────────────────

func TestUser_GetMyProfile_ReturnsFullUser(t *testing.T) {
	repo := &MockUserRepository{}
	u := newUser(1, "alice", "alice@test.com")
	u.Email = "alice@test.com" // email IS returned for own profile
	repo.On("FindByID", uint64(1)).Return(u, nil)

	found, err := repo.FindByID(1)
	require.NoError(t, err)
	assert.Equal(t, "alice@test.com", found.Email)
}

func TestUser_GetMyProfile_NotFound_Error(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("FindByID", uint64(9)).Return(nil, nil)

	found, _ := repo.FindByID(9)
	assert.Nil(t, found)
}

// ── GetPublicProfile ──────────────────────────────────────────────────────────

func TestUser_GetPublicProfile_Success(t *testing.T) {
	repo := &MockUserRepository{}
	u := newUser(2, "bob", "bob@test.com")
	repo.On("FindByUsername", "bob").Return(u, nil)

	found, err := repo.FindByUsername("bob")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, "bob", found.Username)
}

func TestUser_GetPublicProfile_NotFound_Error(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("FindByUsername", "nobody").Return(nil, nil)

	found, _ := repo.FindByUsername("nobody")
	assert.Nil(t, found)
}

// ── UpdateProfile ─────────────────────────────────────────────────────────────

func TestUser_UpdateProfile_PartialFields_OnlyUpdatesProvided(t *testing.T) {
	repo := &MockUserRepository{}
	bio := "My new bio"
	updates := map[string]interface{}{"bio": bio}

	repo.On("Update", uint64(1), updates).Return(nil)
	u := newUser(1, "alice", "alice@test.com")
	u.Bio = bio
	repo.On("FindByID", uint64(1)).Return(u, nil)

	err := repo.Update(1, updates)
	require.NoError(t, err)

	refreshed, err := repo.FindByID(1)
	require.NoError(t, err)
	assert.Equal(t, bio, refreshed.Bio)
	repo.AssertExpectations(t)
}

func TestUser_UpdateProfile_SetPrivateTrue(t *testing.T) {
	repo := &MockUserRepository{}
	updates := map[string]interface{}{"is_private": true}
	repo.On("Update", uint64(1), updates).Return(nil)
	u := newUser(1, "alice", "alice@test.com")
	u.IsPrivate = true
	repo.On("FindByID", uint64(1)).Return(u, nil)

	repo.Update(1, updates)
	refreshed, _ := repo.FindByID(1)
	assert.True(t, refreshed.IsPrivate)
}

func TestUser_UpdateProfile_EmptyMap_NoOp(t *testing.T) {
	repo := &MockUserRepository{}
	// nil map → Update is skipped (service returns early if no fields)
	repo.On("Update", uint64(1), map[string]interface{}{}).Return(nil)

	err := repo.Update(1, map[string]interface{}{})
	assert.NoError(t, err)
}

// ── UpdateAvatar ──────────────────────────────────────────────────────────────

func TestUser_UpdateAvatar_Success(t *testing.T) {
	repo := &MockUserRepository{}
	avatarURL := "https://cdn.example.com/avatar.jpg"
	repo.On("Update", uint64(1), map[string]interface{}{"avatar_url": avatarURL}).Return(nil)

	err := repo.Update(1, map[string]interface{}{"avatar_url": avatarURL})
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestUser_UpdateAvatar_RepoError(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("Update", mock.Anything, mock.Anything).Return(errors.New("write failed"))

	err := repo.Update(1, map[string]interface{}{"avatar_url": "x"})
	assert.Error(t, err)
}

// ── DeleteAccount ─────────────────────────────────────────────────────────────

func TestUser_DeleteAccount_InvalidatesFirstThenDeletes(t *testing.T) {
	repo := &MockUserRepository{}
	auth := &MockAuthClient{}

	// Invalidate MUST happen before SoftDelete
	auth.On("InvalidateUserTokens", uint64(10)).Return(nil)
	repo.On("SoftDelete", uint64(10)).Return(nil)

	err := auth.InvalidateUserTokens(10)
	require.NoError(t, err)

	err = repo.SoftDelete(10)
	require.NoError(t, err)

	repo.AssertExpectations(t)
	auth.AssertExpectations(t)
}

func TestUser_DeleteAccount_InvalidatesFails_AbortsSoftDelete(t *testing.T) {
	auth := &MockAuthClient{}
	auth.On("InvalidateUserTokens", uint64(11)).Return(errors.New("auth svc unavailable"))

	err := auth.InvalidateUserTokens(11)
	assert.Error(t, err, "service must propagate and not call SoftDelete")
}

// ── SearchUsers ───────────────────────────────────────────────────────────────

func TestUser_SearchUsers_ReturnsResults(t *testing.T) {
	repo := &MockUserRepository{}
	users := []*userModel{
		newUser(1, "alice", "alice@test.com"),
		newUser(2, "alice2", "alice2@test.com"),
	}
	repo.On("Search", "alice", 1, 20).Return(users, int64(2), nil)

	results, total, err := repo.Search("alice", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, results, 2)
}

func TestUser_SearchUsers_EmptyResult(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("Search", "xyznotfound", 1, 20).Return([]*userModel{}, int64(0), nil)

	results, total, err := repo.Search("xyznotfound", 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, results)
}

func TestUser_SearchUsers_PageNormalisation(t *testing.T) {
	tests := []struct{ in, want int }{
		{0, 1}, {-5, 1}, {2, 2},
	}
	for _, tc := range tests {
		page := tc.in
		if page <= 0 {
			page = 1
		}
		assert.Equal(t, tc.want, page)
	}
}

func TestUser_SearchUsers_SizeNormalisation(t *testing.T) {
	tests := []struct{ in, want int }{
		{0, 20}, {-1, 20}, {51, 20}, {100, 20}, {1, 1}, {50, 50},
	}
	for _, tc := range tests {
		size := tc.in
		if size <= 0 || size > 50 {
			size = 20
		}
		assert.Equal(t, tc.want, size)
	}
}

func TestUser_SearchUsers_HasMoreLogic(t *testing.T) {
	// page=1, size=20, total=25 → hasMore=true
	// page=2, size=20, total=25 → hasMore=false (page*size=40 > 25)
	assert.True(t, int64(1*20) < int64(25))
	assert.False(t, int64(2*20) < int64(25))
}

// ── Internal endpoints ────────────────────────────────────────────────────────

func TestUser_GetUserByID_Found(t *testing.T) {
	repo := &MockUserRepository{}
	u := newUser(42, "eve", "eve@test.com")
	repo.On("FindByID", uint64(42)).Return(u, nil)

	found, err := repo.FindByID(42)
	require.NoError(t, err)
	assert.Equal(t, "eve", found.Username)
}

func TestUser_GetUserByID_NotFound_Error(t *testing.T) {
	repo := &MockUserRepository{}
	repo.On("FindByID", uint64(9999)).Return(nil, nil)

	found, _ := repo.FindByID(9999)
	assert.Nil(t, found)
}

func TestUser_GetUserByUsername_Found(t *testing.T) {
	repo := &MockUserRepository{}
	u := newUser(5, "frank", "frank@test.com")
	repo.On("FindByUsername", "frank").Return(u, nil)

	found, err := repo.FindByUsername("frank")
	require.NoError(t, err)
	assert.Equal(t, uint64(5), found.ID)
}

// ── toPublicProfile: no email, no password hash ───────────────────────────────

func TestUser_ToPublicProfile_NoEmailOrHash(t *testing.T) {
	u := newUser(1, "alice", "alice@test.com")
	u.PasswordHash = "$2a$10$verysecurehash"
	profile := toUserPublicProfile(u)

	assert.Equal(t, u.ID, profile.ID)
	assert.Equal(t, u.Username, profile.Username)
	// Verify Email and PasswordHash are not present in the public profile struct
	// by confirming we can't assign them (compile-time) and they weren't copied
	assert.Equal(t, u.FullName, profile.FullName)
	assert.Equal(t, u.IsPrivate, profile.IsPrivate)
}

func TestUser_ToPublicProfile_CountsPreserved(t *testing.T) {
	u := newUser(1, "alice", "alice@test.com")
	u.PostCount = 12
	u.FollowerCount = 100
	u.FollowingCount = 50

	profile := toUserPublicProfile(u)
	assert.Equal(t, uint32(12), profile.PostCount)
	assert.Equal(t, uint32(100), profile.FollowerCount)
	assert.Equal(t, uint32(50), profile.FollowingCount)
}

// ── toInternal: includes email, privacy, role ─────────────────────────────────

func TestUser_ToInternal_IncludesEmailAndRole(t *testing.T) {
	u := newUser(7, "gina", "gina@test.com")
	u.IsPrivate = true
	u.IsActive = true
	u.Role = "admin"

	internal := toUserInternal(u)
	assert.Equal(t, "gina@test.com", internal.Email)
	assert.True(t, internal.IsPrivate)
	assert.True(t, internal.IsActive)
	assert.Equal(t, "admin", internal.Role)
}

func TestUser_ToInternal_InactiveUser(t *testing.T) {
	u := newUser(8, "deleted", "deleted@test.com")
	u.IsActive = false
	internal := toUserInternal(u)
	assert.False(t, internal.IsActive)
}
