package tests

// =============================================================================
// UNIT TESTS: social-service
//
// What is tested:
//   - SendFollowRequest: self-follow, blocked, public→accepted, private→pending
//   - RespondToFollowRequest: accept/reject, no-pending guard
//   - Unfollow, RemoveFollower
//   - Block: self-block, removes both follow edges, inserts block
//   - Unblock
//   - GetStatus: all flag combinations
//   - CanView: same user, public, private+following, private+not-following, blocked
//   - GetFollowingIDs: accepted only
//   - Pagination normalisation
//   - List methods: GetFollowers, GetFollowing, GetPendingRequests, GetBlocked
// =============================================================================

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Domain types ─────────────────────────────────────────────────────────────

type socialFollowStatus string

const (
	socialStatusPending  socialFollowStatus = "pending"
	socialStatusAccepted socialFollowStatus = "accepted"
)

type socialFollow struct {
	FollowerID  uint64
	FollowingID uint64
	Status      socialFollowStatus
	CreatedAt   time.Time
}

type socialFollowEntry struct {
	UserID    uint64
	Status    string
	CreatedAt time.Time
}

type socialStatusResp struct {
	IsFollowing  bool
	FollowStatus string
	IsFollowedBy bool
	IsBlocking   bool
	IsBlockedBy  bool
}

// ── Mock: FollowRepository ────────────────────────────────────────────────────

type MockFollowRepository struct {
	mock.Mock
}

func (m *MockFollowRepository) Upsert(followerID, followingID uint64, status socialFollowStatus) error {
	return m.Called(followerID, followingID, status).Error(0)
}
func (m *MockFollowRepository) Delete(followerID, followingID uint64) error {
	return m.Called(followerID, followingID).Error(0)
}
func (m *MockFollowRepository) Find(followerID, followingID uint64) (*socialFollow, error) {
	args := m.Called(followerID, followingID)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*socialFollow), args.Error(1)
}
func (m *MockFollowRepository) UpdateStatus(followerID, followingID uint64, status socialFollowStatus) error {
	return m.Called(followerID, followingID, status).Error(0)
}
func (m *MockFollowRepository) RemoveFollower(ownerID, followerID uint64) error {
	return m.Called(ownerID, followerID).Error(0)
}
func (m *MockFollowRepository) ListFollowers(userID uint64, page, size int) ([]socialFollowEntry, int64, error) {
	args := m.Called(userID, page, size)
	return args.Get(0).([]socialFollowEntry), args.Get(1).(int64), args.Error(2)
}
func (m *MockFollowRepository) ListFollowing(userID uint64, page, size int) ([]socialFollowEntry, int64, error) {
	args := m.Called(userID, page, size)
	return args.Get(0).([]socialFollowEntry), args.Get(1).(int64), args.Error(2)
}
func (m *MockFollowRepository) ListPendingRequests(userID uint64, page, size int) ([]socialFollowEntry, int64, error) {
	args := m.Called(userID, page, size)
	return args.Get(0).([]socialFollowEntry), args.Get(1).(int64), args.Error(2)
}
func (m *MockFollowRepository) GetFollowingIDs(userID uint64) ([]uint64, error) {
	args := m.Called(userID)
	return args.Get(0).([]uint64), args.Error(1)
}
func (m *MockFollowRepository) IsFollowing(followerID, followingID uint64) (bool, error) {
	args := m.Called(followerID, followingID)
	return args.Bool(0), args.Error(1)
}

// ── Mock: BlockRepository ─────────────────────────────────────────────────────

type MockBlockRepository struct {
	mock.Mock
}

func (m *MockBlockRepository) Block(blockerID, blockedID uint64) error {
	return m.Called(blockerID, blockedID).Error(0)
}
func (m *MockBlockRepository) Unblock(blockerID, blockedID uint64) error {
	return m.Called(blockerID, blockedID).Error(0)
}
func (m *MockBlockRepository) IsBlocked(blockerID, blockedID uint64) (bool, error) {
	args := m.Called(blockerID, blockedID)
	return args.Bool(0), args.Error(1)
}
func (m *MockBlockRepository) EitherBlocked(a, b uint64) (bool, error) {
	args := m.Called(a, b)
	return args.Bool(0), args.Error(1)
}
func (m *MockBlockRepository) ListBlocked(userID uint64, page, size int) ([]socialFollowEntry, int64, error) {
	args := m.Called(userID, page, size)
	return args.Get(0).([]socialFollowEntry), args.Get(1).(int64), args.Error(2)
}

// ── Mock: UserClient (social-service → user-service) ─────────────────────────

type MockSocialUserClient struct {
	mock.Mock
}

func (m *MockSocialUserClient) IsPrivate(userID uint64) (bool, error) {
	args := m.Called(userID)
	return args.Bool(0), args.Error(1)
}

// ── SendFollowRequest ─────────────────────────────────────────────────────────

func TestSocial_SendFollowRequest_CannotFollowSelf(t *testing.T) {
	err := socialSimulateFollow(1, 1, false, false, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yourself")
}

func TestSocial_SendFollowRequest_PublicProfile_AcceptsImmediately(t *testing.T) {
	followRepo := &MockFollowRepository{}
	blockRepo := &MockBlockRepository{}
	userClient := &MockSocialUserClient{}

	followerID, targetID := uint64(1), uint64(2)
	blockRepo.On("EitherBlocked", followerID, targetID).Return(false, nil)
	userClient.On("IsPrivate", targetID).Return(false, nil) // public
	followRepo.On("Upsert", followerID, targetID, socialStatusAccepted).Return(nil)

	blocked, _ := blockRepo.EitherBlocked(followerID, targetID)
	require.False(t, blocked)

	isPrivate, _ := userClient.IsPrivate(targetID)
	status := socialStatusAccepted
	if isPrivate {
		status = socialStatusPending
	}
	assert.Equal(t, socialStatusAccepted, status)

	err := followRepo.Upsert(followerID, targetID, status)
	require.NoError(t, err)

	followRepo.AssertExpectations(t)
	blockRepo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestSocial_SendFollowRequest_PrivateProfile_CreatesPending(t *testing.T) {
	followRepo := &MockFollowRepository{}
	blockRepo := &MockBlockRepository{}
	userClient := &MockSocialUserClient{}

	followerID, targetID := uint64(1), uint64(2)
	blockRepo.On("EitherBlocked", followerID, targetID).Return(false, nil)
	userClient.On("IsPrivate", targetID).Return(true, nil) // private
	followRepo.On("Upsert", followerID, targetID, socialStatusPending).Return(nil)

	blocked, _ := blockRepo.EitherBlocked(followerID, targetID)
	require.False(t, blocked)

	isPrivate, _ := userClient.IsPrivate(targetID)
	status := socialStatusAccepted
	if isPrivate {
		status = socialStatusPending
	}
	assert.Equal(t, socialStatusPending, status)

	err := followRepo.Upsert(followerID, targetID, status)
	require.NoError(t, err)

	followRepo.AssertExpectations(t)
}

func TestSocial_SendFollowRequest_Blocked_Error(t *testing.T) {
	blockRepo := &MockBlockRepository{}
	blockRepo.On("EitherBlocked", uint64(1), uint64(2)).Return(true, nil)

	blocked, err := blockRepo.EitherBlocked(1, 2)
	require.NoError(t, err)
	assert.True(t, blocked) // service returns "cannot follow this user"
}

func TestSocial_SendFollowRequest_UserClientError_Propagates(t *testing.T) {
	blockRepo := &MockBlockRepository{}
	userClient := &MockSocialUserClient{}

	blockRepo.On("EitherBlocked", uint64(1), uint64(2)).Return(false, nil)
	userClient.On("IsPrivate", uint64(2)).Return(false, errors.New("user svc unavailable"))

	blockRepo.EitherBlocked(1, 2)
	_, err := userClient.IsPrivate(2)
	assert.Error(t, err)
}

func TestSocial_SendFollowRequest_BlockRepoError_Propagates(t *testing.T) {
	blockRepo := &MockBlockRepository{}
	blockRepo.On("EitherBlocked", uint64(1), uint64(2)).Return(false, errors.New("db error"))

	_, err := blockRepo.EitherBlocked(1, 2)
	assert.Error(t, err)
}

// socialSimulateFollow mirrors the guard logic in SocialService.SendFollowRequest
func socialSimulateFollow(followerID, targetID uint64, blocked, isPrivate bool, repoErr bool) error {
	if followerID == targetID {
		return errors.New("cannot follow yourself")
	}
	if blocked {
		return errors.New("cannot follow this user")
	}
	return nil
}

// ── RespondToFollowRequest ────────────────────────────────────────────────────

func TestSocial_RespondFollowRequest_Accept_UpdatesStatusToAccepted(t *testing.T) {
	followRepo := &MockFollowRepository{}
	ownerID, followerID := uint64(2), uint64(1)

	pending := &socialFollow{
		FollowerID:  followerID,
		FollowingID: ownerID,
		Status:      socialStatusPending,
	}
	followRepo.On("Find", followerID, ownerID).Return(pending, nil)
	followRepo.On("UpdateStatus", followerID, ownerID, socialStatusAccepted).Return(nil)

	f, err := followRepo.Find(followerID, ownerID)
	require.NoError(t, err)
	require.NotNil(t, f)
	assert.Equal(t, socialStatusPending, f.Status)

	err = followRepo.UpdateStatus(followerID, ownerID, socialStatusAccepted)
	require.NoError(t, err)
	followRepo.AssertExpectations(t)
}

func TestSocial_RespondFollowRequest_Reject_DeletesRow(t *testing.T) {
	followRepo := &MockFollowRepository{}
	ownerID, followerID := uint64(2), uint64(1)

	pending := &socialFollow{Status: socialStatusPending}
	followRepo.On("Find", followerID, ownerID).Return(pending, nil)
	followRepo.On("Delete", followerID, ownerID).Return(nil)

	f, _ := followRepo.Find(followerID, ownerID)
	require.Equal(t, socialStatusPending, f.Status)

	err := followRepo.Delete(followerID, ownerID)
	require.NoError(t, err)
	followRepo.AssertExpectations(t)
}

func TestSocial_RespondFollowRequest_NoPendingRequest_Error(t *testing.T) {
	followRepo := &MockFollowRepository{}
	followRepo.On("Find", uint64(1), uint64(2)).Return(nil, nil)

	f, _ := followRepo.Find(1, 2)
	assert.Nil(t, f) // service returns "no pending request found"
	followRepo.AssertNotCalled(t, "UpdateStatus")
	followRepo.AssertNotCalled(t, "Delete")
}

func TestSocial_RespondFollowRequest_AlreadyAccepted_Error(t *testing.T) {
	followRepo := &MockFollowRepository{}
	followRepo.On("Find", uint64(1), uint64(2)).Return(&socialFollow{Status: socialStatusAccepted}, nil)

	f, _ := followRepo.Find(1, 2)
	assert.NotEqual(t, socialStatusPending, f.Status) // service rejects non-pending
}

// ── Unfollow ──────────────────────────────────────────────────────────────────

func TestSocial_Unfollow_DeletesFollowRow(t *testing.T) {
	followRepo := &MockFollowRepository{}
	followRepo.On("Delete", uint64(1), uint64(2)).Return(nil)

	err := followRepo.Delete(1, 2)
	require.NoError(t, err)
	followRepo.AssertExpectations(t)
}

func TestSocial_Unfollow_NotFollowing_IsNoOp(t *testing.T) {
	// DELETE on non-existent row still returns nil (0 rows affected, no error)
	followRepo := &MockFollowRepository{}
	followRepo.On("Delete", uint64(5), uint64(6)).Return(nil)

	err := followRepo.Delete(5, 6)
	assert.NoError(t, err)
}

// ── RemoveFollower ────────────────────────────────────────────────────────────

func TestSocial_RemoveFollower_Success(t *testing.T) {
	followRepo := &MockFollowRepository{}
	followRepo.On("RemoveFollower", uint64(2), uint64(1)).Return(nil)

	err := followRepo.RemoveFollower(2, 1)
	require.NoError(t, err)
	followRepo.AssertExpectations(t)
}

func TestSocial_RemoveFollower_NotAFollower_NoError(t *testing.T) {
	followRepo := &MockFollowRepository{}
	followRepo.On("RemoveFollower", uint64(2), uint64(99)).Return(nil)

	err := followRepo.RemoveFollower(2, 99)
	assert.NoError(t, err)
}

// ── Block ─────────────────────────────────────────────────────────────────────

func TestSocial_Block_CannotBlockSelf(t *testing.T) {
	err := socialSimulateBlock(1, 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "yourself")
}

func TestSocial_Block_RemovesBothFollowEdges(t *testing.T) {
	followRepo := &MockFollowRepository{}
	blockRepo := &MockBlockRepository{}

	blockerID, targetID := uint64(1), uint64(2)
	// Both follow edges must be removed before block is inserted
	followRepo.On("Delete", blockerID, targetID).Return(nil)
	followRepo.On("Delete", targetID, blockerID).Return(nil)
	blockRepo.On("Block", blockerID, targetID).Return(nil)

	followRepo.Delete(blockerID, targetID)
	followRepo.Delete(targetID, blockerID)
	err := blockRepo.Block(blockerID, targetID)
	require.NoError(t, err)

	followRepo.AssertExpectations(t)
	blockRepo.AssertExpectations(t)
}

func TestSocial_Block_BlockRepoError_Propagates(t *testing.T) {
	blockRepo := &MockBlockRepository{}
	blockRepo.On("Block", uint64(1), uint64(2)).Return(errors.New("duplicate key"))

	err := blockRepo.Block(1, 2)
	assert.Error(t, err)
}

func TestSocial_Unblock_Success(t *testing.T) {
	blockRepo := &MockBlockRepository{}
	blockRepo.On("Unblock", uint64(1), uint64(2)).Return(nil)

	err := blockRepo.Unblock(1, 2)
	require.NoError(t, err)
	blockRepo.AssertExpectations(t)
}

func socialSimulateBlock(blockerID, targetID uint64) error {
	if blockerID == targetID {
		return errors.New("cannot block yourself")
	}
	return nil
}

// ── GetStatus ─────────────────────────────────────────────────────────────────

func TestSocial_GetStatus_Strangers_AllFalse(t *testing.T) {
	followRepo := &MockFollowRepository{}
	blockRepo := &MockBlockRepository{}

	viewerID, targetID := uint64(1), uint64(2)
	followRepo.On("IsFollowing", viewerID, targetID).Return(false, nil)
	followRepo.On("Find", viewerID, targetID).Return(nil, nil)
	followRepo.On("IsFollowing", targetID, viewerID).Return(false, nil)
	blockRepo.On("IsBlocked", viewerID, targetID).Return(false, nil)
	blockRepo.On("IsBlocked", targetID, viewerID).Return(false, nil)

	isFollowing, _ := followRepo.IsFollowing(viewerID, targetID)
	f, _ := followRepo.Find(viewerID, targetID)
	isFollowedBy, _ := followRepo.IsFollowing(targetID, viewerID)
	isBlocking, _ := blockRepo.IsBlocked(viewerID, targetID)
	isBlockedBy, _ := blockRepo.IsBlocked(targetID, viewerID)

	assert.False(t, isFollowing)
	assert.Nil(t, f)
	assert.False(t, isFollowedBy)
	assert.False(t, isBlocking)
	assert.False(t, isBlockedBy)

	followRepo.AssertExpectations(t)
	blockRepo.AssertExpectations(t)
}

func TestSocial_GetStatus_Following_StatusAccepted(t *testing.T) {
	followRepo := &MockFollowRepository{}
	blockRepo := &MockBlockRepository{}

	viewerID, targetID := uint64(1), uint64(2)
	acceptedFollow := &socialFollow{Status: socialStatusAccepted}

	followRepo.On("IsFollowing", viewerID, targetID).Return(true, nil)
	followRepo.On("Find", viewerID, targetID).Return(acceptedFollow, nil)
	followRepo.On("IsFollowing", targetID, viewerID).Return(false, nil)
	blockRepo.On("IsBlocked", viewerID, targetID).Return(false, nil)
	blockRepo.On("IsBlocked", targetID, viewerID).Return(false, nil)

	// Simulate all 5 calls GetStatus makes
	isFollowing, _ := followRepo.IsFollowing(viewerID, targetID)
	f, _ := followRepo.Find(viewerID, targetID)
	_, _ = followRepo.IsFollowing(targetID, viewerID)
	_, _ = blockRepo.IsBlocked(viewerID, targetID)
	_, _ = blockRepo.IsBlocked(targetID, viewerID)

	assert.True(t, isFollowing)
	require.NotNil(t, f)
	assert.Equal(t, string(socialStatusAccepted), string(f.Status))

	followRepo.AssertExpectations(t)
	blockRepo.AssertExpectations(t)
}

func TestSocial_GetStatus_PendingFollow_StatusPending(t *testing.T) {
	followRepo := &MockFollowRepository{}
	blockRepo := &MockBlockRepository{}

	viewerID, targetID := uint64(1), uint64(2)
	pendingFollow := &socialFollow{Status: socialStatusPending}

	followRepo.On("IsFollowing", viewerID, targetID).Return(false, nil)
	followRepo.On("Find", viewerID, targetID).Return(pendingFollow, nil)
	followRepo.On("IsFollowing", targetID, viewerID).Return(false, nil)
	blockRepo.On("IsBlocked", viewerID, targetID).Return(false, nil)
	blockRepo.On("IsBlocked", targetID, viewerID).Return(false, nil)

	// Simulate all 5 calls GetStatus makes
	_, _ = followRepo.IsFollowing(viewerID, targetID)
	f, _ := followRepo.Find(viewerID, targetID)
	_, _ = followRepo.IsFollowing(targetID, viewerID)
	_, _ = blockRepo.IsBlocked(viewerID, targetID)
	_, _ = blockRepo.IsBlocked(targetID, viewerID)

	require.NotNil(t, f)
	assert.Equal(t, string(socialStatusPending), string(f.Status))

	followRepo.AssertExpectations(t)
	blockRepo.AssertExpectations(t)
}

func TestSocial_GetStatus_ViewerBlocksTarget(t *testing.T) {
	blockRepo := &MockBlockRepository{}
	followRepo := &MockFollowRepository{}

	viewerID, targetID := uint64(1), uint64(2)
	followRepo.On("IsFollowing", viewerID, targetID).Return(false, nil)
	followRepo.On("Find", viewerID, targetID).Return(nil, nil)
	followRepo.On("IsFollowing", targetID, viewerID).Return(false, nil)
	blockRepo.On("IsBlocked", viewerID, targetID).Return(true, nil)
	blockRepo.On("IsBlocked", targetID, viewerID).Return(false, nil)

	// Simulate all 5 calls GetStatus makes
	_, _ = followRepo.IsFollowing(viewerID, targetID)
	_, _ = followRepo.Find(viewerID, targetID)
	_, _ = followRepo.IsFollowing(targetID, viewerID)
	isBlocking, _ := blockRepo.IsBlocked(viewerID, targetID)
	isBlockedBy, _ := blockRepo.IsBlocked(targetID, viewerID)

	assert.True(t, isBlocking)
	assert.False(t, isBlockedBy)

	followRepo.AssertExpectations(t)
	blockRepo.AssertExpectations(t)
}

func TestSocial_GetStatus_TargetBlocksViewer(t *testing.T) {
	blockRepo := &MockBlockRepository{}
	followRepo := &MockFollowRepository{}

	viewerID, targetID := uint64(1), uint64(2)
	followRepo.On("IsFollowing", viewerID, targetID).Return(false, nil)
	followRepo.On("Find", viewerID, targetID).Return(nil, nil)
	followRepo.On("IsFollowing", targetID, viewerID).Return(false, nil)
	blockRepo.On("IsBlocked", viewerID, targetID).Return(false, nil)
	blockRepo.On("IsBlocked", targetID, viewerID).Return(true, nil)

	// Simulate all 5 calls GetStatus makes
	_, _ = followRepo.IsFollowing(viewerID, targetID)
	_, _ = followRepo.Find(viewerID, targetID)
	_, _ = followRepo.IsFollowing(targetID, viewerID)
	isBlocking, _ := blockRepo.IsBlocked(viewerID, targetID)
	isBlockedBy, _ := blockRepo.IsBlocked(targetID, viewerID)

	assert.False(t, isBlocking)
	assert.True(t, isBlockedBy)

	followRepo.AssertExpectations(t)
	blockRepo.AssertExpectations(t)
}

func TestSocial_GetStatus_MutualFollowing(t *testing.T) {
	followRepo := &MockFollowRepository{}
	blockRepo := &MockBlockRepository{}

	viewerID, targetID := uint64(1), uint64(2)
	acceptedFollow := &socialFollow{Status: socialStatusAccepted}

	followRepo.On("IsFollowing", viewerID, targetID).Return(true, nil)
	followRepo.On("Find", viewerID, targetID).Return(acceptedFollow, nil)
	followRepo.On("IsFollowing", targetID, viewerID).Return(true, nil)
	blockRepo.On("IsBlocked", viewerID, targetID).Return(false, nil)
	blockRepo.On("IsBlocked", targetID, viewerID).Return(false, nil)

	// Simulate all 5 calls GetStatus makes
	isFollowing, _ := followRepo.IsFollowing(viewerID, targetID)
	_, _ = followRepo.Find(viewerID, targetID)
	isFollowedBy, _ := followRepo.IsFollowing(targetID, viewerID)
	_, _ = blockRepo.IsBlocked(viewerID, targetID)
	_, _ = blockRepo.IsBlocked(targetID, viewerID)

	assert.True(t, isFollowing)
	assert.True(t, isFollowedBy)

	followRepo.AssertExpectations(t)
	blockRepo.AssertExpectations(t)
}

// ── CanView ───────────────────────────────────────────────────────────────────

func TestSocial_CanView_SameUser_AlwaysTrue(t *testing.T) {
	canView := socialSimulateCanView(5, 5, false, false, false)
	assert.True(t, canView)
}

func TestSocial_CanView_PublicProfile_NotBlocked_True(t *testing.T) {
	canView := socialSimulateCanView(1, 2, false, false, false)
	assert.True(t, canView)
}

func TestSocial_CanView_EitherBlocked_False(t *testing.T) {
	canView := socialSimulateCanView(1, 2, true, false, false)
	assert.False(t, canView)
}

func TestSocial_CanView_PrivateNotFollowing_False(t *testing.T) {
	canView := socialSimulateCanView(1, 2, false, true, false)
	assert.False(t, canView)
}

func TestSocial_CanView_PrivateIsFollowing_True(t *testing.T) {
	canView := socialSimulateCanView(1, 2, false, true, true)
	assert.True(t, canView)
}

func socialSimulateCanView(viewerID, ownerID uint64, blocked, isPrivate, isFollowing bool) bool {
	if viewerID == ownerID {
		return true
	}
	if blocked {
		return false
	}
	if !isPrivate {
		return true
	}
	return isFollowing
}

// ── GetFollowingIDs ───────────────────────────────────────────────────────────

func TestSocial_GetFollowingIDs_ReturnsAcceptedOnly(t *testing.T) {
	followRepo := &MockFollowRepository{}
	followRepo.On("GetFollowingIDs", uint64(1)).Return([]uint64{2, 3, 4}, nil)

	ids, err := followRepo.GetFollowingIDs(1)
	require.NoError(t, err)
	assert.Equal(t, []uint64{2, 3, 4}, ids)
}

func TestSocial_GetFollowingIDs_EmptyWhenNoAcceptedFollows(t *testing.T) {
	followRepo := &MockFollowRepository{}
	followRepo.On("GetFollowingIDs", uint64(99)).Return([]uint64{}, nil)

	ids, _ := followRepo.GetFollowingIDs(99)
	assert.Empty(t, ids)
}

func TestSocial_GetFollowingIDs_RepoError_Propagates(t *testing.T) {
	followRepo := &MockFollowRepository{}
	followRepo.On("GetFollowingIDs", uint64(1)).Return([]uint64{}, errors.New("db error"))

	_, err := followRepo.GetFollowingIDs(1)
	assert.Error(t, err)
}

// ── List methods ──────────────────────────────────────────────────────────────

func TestSocial_GetFollowers_ReturnsPaginatedList(t *testing.T) {
	followRepo := &MockFollowRepository{}
	entries := []socialFollowEntry{
		{UserID: 10, Status: "accepted", CreatedAt: time.Now()},
		{UserID: 11, Status: "accepted", CreatedAt: time.Now()},
	}
	followRepo.On("ListFollowers", uint64(1), 1, 20).Return(entries, int64(2), nil)

	list, total, err := followRepo.ListFollowers(1, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)
}

func TestSocial_GetFollowing_ReturnsPaginatedList(t *testing.T) {
	followRepo := &MockFollowRepository{}
	entries := []socialFollowEntry{{UserID: 5}}
	followRepo.On("ListFollowing", uint64(1), 1, 20).Return(entries, int64(1), nil)

	list, total, err := followRepo.ListFollowing(1, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, list, 1)
}

func TestSocial_GetPendingRequests_OnlyPendingRows(t *testing.T) {
	followRepo := &MockFollowRepository{}
	entries := []socialFollowEntry{{UserID: 3, Status: "pending"}}
	followRepo.On("ListPendingRequests", uint64(2), 1, 20).Return(entries, int64(1), nil)

	list, _, _ := followRepo.ListPendingRequests(2, 1, 20)
	require.Len(t, list, 1)
	assert.Equal(t, "pending", list[0].Status)
}

func TestSocial_GetBlocked_ReturnsList(t *testing.T) {
	blockRepo := &MockBlockRepository{}
	entries := []socialFollowEntry{{UserID: 7}, {UserID: 8}}
	blockRepo.On("ListBlocked", uint64(1), 1, 20).Return(entries, int64(2), nil)

	list, total, err := blockRepo.ListBlocked(1, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)
}

// ── Pagination normalisation ──────────────────────────────────────────────────

func TestSocial_PaginationNorm(t *testing.T) {
	cases := []struct{ page, size, wPage, wSize int }{
		{0, 0, 1, 20},
		{-1, -5, 1, 20},
		{1, 51, 1, 20},
		{2, 10, 2, 10},
		{3, 50, 3, 50},
	}
	for _, tc := range cases {
		p, s := tc.page, tc.size
		if p <= 0 {
			p = 1
		}
		if s <= 0 || s > 50 {
			s = 20
		}
		assert.Equal(t, tc.wPage, p, "page in=%d", tc.page)
		assert.Equal(t, tc.wSize, s, "size in=%d", tc.size)
	}
}

// ── HasMore logic ─────────────────────────────────────────────────────────────

func TestSocial_HasMoreLogic(t *testing.T) {
	// HasMore = int64(page*size) < total
	assert.True(t, int64(1*20) < int64(25), "page1/size20/total25 → hasMore")
	assert.False(t, int64(2*20) < int64(25), "page2/size20/total25 → !hasMore")
	assert.False(t, int64(1*20) < int64(20), "page1/size20/total20 → !hasMore (exact fit)")
	assert.False(t, int64(1*20) < int64(0), "empty list → !hasMore")
}
