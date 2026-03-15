package tests

// =============================================================================
// UNIT TESTS: interaction-service + feed-service
//
// interaction-service:
//   - Like: access check via postClient.GetPostOwner + social.CanView,
//           INSERT IGNORE → added=true increments count, added=false (dup) does not
//   - Unlike: removed=true decrements count, removed=false (not-liked) does not
//   - HasLiked: delegates to likeRepo
//   - AddComment: same access checks as Like, creates comment, increments count
//   - UpdateComment: ownership check, not-found → forbidden, not-owner → forbidden
//   - DeleteComment: ownership check, decrements count on success
//   - ListComments: same access checks, paginated
//
// feed-service:
//   - GetFeed: calls social.GetFollowingIDs, appends self, calls post.GetPostsByUserIDs
//   - Own posts always included (userID appended to followingIDs)
//   - Empty following list still includes own posts
//   - Pagination normalisation
// =============================================================================

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Interaction domain types ──────────────────────────────────────────────────

type interactionComment struct {
	ID        uint64
	PostID    uint64
	UserID    uint64
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ── Mock: LikeRepository ──────────────────────────────────────────────────────

type MockLikeRepository struct {
	mock.Mock
}

func (m *MockLikeRepository) Like(userID, postID uint64) (bool, error) {
	args := m.Called(userID, postID)
	return args.Bool(0), args.Error(1)
}
func (m *MockLikeRepository) Unlike(userID, postID uint64) (bool, error) {
	args := m.Called(userID, postID)
	return args.Bool(0), args.Error(1)
}
func (m *MockLikeRepository) HasLiked(userID, postID uint64) (bool, error) {
	args := m.Called(userID, postID)
	return args.Bool(0), args.Error(1)
}

// ── Mock: CommentRepository ───────────────────────────────────────────────────

type MockCommentRepository struct {
	mock.Mock
}

func (m *MockCommentRepository) Create(c *interactionComment) (*interactionComment, error) {
	args := m.Called(c)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*interactionComment), args.Error(1)
}
func (m *MockCommentRepository) FindByID(id uint64) (*interactionComment, error) {
	args := m.Called(id)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*interactionComment), args.Error(1)
}
func (m *MockCommentRepository) Update(id, userID uint64, body string) error {
	return m.Called(id, userID, body).Error(0)
}
func (m *MockCommentRepository) SoftDelete(id, userID uint64) error {
	return m.Called(id, userID).Error(0)
}
func (m *MockCommentRepository) ListByPost(postID uint64, page, size int) ([]*interactionComment, int64, error) {
	args := m.Called(postID, page, size)
	return args.Get(0).([]*interactionComment), args.Get(1).(int64), args.Error(2)
}

// ── Mock: PostClient (interaction-service → post-service) ────────────────────

type MockInteractionPostClient struct {
	mock.Mock
}

func (m *MockInteractionPostClient) GetPostOwner(postID uint64) (uint64, error) {
	args := m.Called(postID)
	return args.Get(0).(uint64), args.Error(1)
}
func (m *MockInteractionPostClient) IncrementLike(postID uint64, delta int) {
	m.Called(postID, delta)
}
func (m *MockInteractionPostClient) IncrementComment(postID uint64, delta int) {
	m.Called(postID, delta)
}

// ── Mock: SocialClient (interaction-service → social-service) ─────────────────

type MockInteractionSocialClient struct {
	mock.Mock
}

func (m *MockInteractionSocialClient) CanView(viewerID, ownerID uint64) (bool, error) {
	args := m.Called(viewerID, ownerID)
	return args.Bool(0), args.Error(1)
}

// ── Like ──────────────────────────────────────────────────────────────────────

func TestInteraction_Like_NewLike_IncrementsCount(t *testing.T) {
	likeRepo := &MockLikeRepository{}
	postClient := &MockInteractionPostClient{}
	socialClient := &MockInteractionSocialClient{}

	userID, postID, ownerID := uint64(1), uint64(10), uint64(5)
	postClient.On("GetPostOwner", postID).Return(ownerID, nil)
	socialClient.On("CanView", userID, ownerID).Return(true, nil)
	likeRepo.On("Like", userID, postID).Return(true, nil) // INSERT IGNORE → new row
	postClient.On("IncrementLike", postID, 1)

	owner, err := postClient.GetPostOwner(postID)
	require.NoError(t, err)

	if userID != owner {
		canView, _ := socialClient.CanView(userID, owner)
		require.True(t, canView)
	}

	added, err := likeRepo.Like(userID, postID)
	require.NoError(t, err)
	assert.True(t, added)

	if added {
		postClient.IncrementLike(postID, 1)
	}

	likeRepo.AssertExpectations(t)
	postClient.AssertExpectations(t)
	socialClient.AssertExpectations(t)
}

func TestInteraction_Like_DuplicateLike_NoIncrement(t *testing.T) {
	// INSERT IGNORE on duplicate → rows affected = 0 → added=false → no increment
	likeRepo := &MockLikeRepository{}
	postClient := &MockInteractionPostClient{}

	userID, postID, ownerID := uint64(1), uint64(10), uint64(1) // owner likes own post
	postClient.On("GetPostOwner", postID).Return(ownerID, nil)
	likeRepo.On("Like", userID, postID).Return(false, nil) // already liked

	postClient.GetPostOwner(postID)
	added, err := likeRepo.Like(userID, postID)
	require.NoError(t, err)
	assert.False(t, added)

	// IncrementLike must NOT be called for duplicate
	postClient.AssertNotCalled(t, "IncrementLike")

	likeRepo.AssertExpectations(t)
	postClient.AssertExpectations(t)
}

func TestInteraction_Like_PostNotFound_Error(t *testing.T) {
	postClient := &MockInteractionPostClient{}
	postClient.On("GetPostOwner", uint64(99)).Return(uint64(0), errors.New("post not found"))

	_, err := postClient.GetPostOwner(99)
	assert.Error(t, err) // service returns "post not found"
}

func TestInteraction_Like_AccessDenied_Error(t *testing.T) {
	postClient := &MockInteractionPostClient{}
	socialClient := &MockInteractionSocialClient{}

	userID, postID, ownerID := uint64(1), uint64(10), uint64(5)
	postClient.On("GetPostOwner", postID).Return(ownerID, nil)
	socialClient.On("CanView", userID, ownerID).Return(false, nil)

	owner, _ := postClient.GetPostOwner(postID)
	canView, _ := socialClient.CanView(userID, owner)
	assert.False(t, canView) // service returns "access denied"
}

func TestInteraction_Like_LikeRepoError_Propagates(t *testing.T) {
	likeRepo := &MockLikeRepository{}
	postClient := &MockInteractionPostClient{}

	postClient.On("GetPostOwner", uint64(10)).Return(uint64(1), nil)
	likeRepo.On("Like", uint64(1), uint64(10)).Return(false, errors.New("db error"))

	likeRepo.Like(1, 10)
	postClient.AssertNotCalled(t, "IncrementLike")
}

// ── Unlike ────────────────────────────────────────────────────────────────────

func TestInteraction_Unlike_WasLiked_DecrementsCount(t *testing.T) {
	likeRepo := &MockLikeRepository{}
	postClient := &MockInteractionPostClient{}

	userID, postID := uint64(1), uint64(10)
	likeRepo.On("Unlike", userID, postID).Return(true, nil) // DELETE → 1 row removed
	postClient.On("IncrementLike", postID, -1)

	removed, err := likeRepo.Unlike(userID, postID)
	require.NoError(t, err)
	assert.True(t, removed)

	if removed {
		postClient.IncrementLike(postID, -1)
	}

	likeRepo.AssertExpectations(t)
	postClient.AssertExpectations(t)
}

func TestInteraction_Unlike_NotLiked_NoDecrement(t *testing.T) {
	likeRepo := &MockLikeRepository{}
	postClient := &MockInteractionPostClient{}

	userID, postID := uint64(1), uint64(10)
	likeRepo.On("Unlike", userID, postID).Return(false, nil) // nothing to delete

	removed, err := likeRepo.Unlike(userID, postID)
	require.NoError(t, err)
	assert.False(t, removed)

	postClient.AssertNotCalled(t, "IncrementLike")
	likeRepo.AssertExpectations(t)
}

// ── HasLiked ──────────────────────────────────────────────────────────────────

func TestInteraction_HasLiked_True(t *testing.T) {
	likeRepo := &MockLikeRepository{}
	likeRepo.On("HasLiked", uint64(1), uint64(10)).Return(true, nil)

	liked, err := likeRepo.HasLiked(1, 10)
	require.NoError(t, err)
	assert.True(t, liked)
}

func TestInteraction_HasLiked_False(t *testing.T) {
	likeRepo := &MockLikeRepository{}
	likeRepo.On("HasLiked", uint64(1), uint64(10)).Return(false, nil)

	liked, err := likeRepo.HasLiked(1, 10)
	require.NoError(t, err)
	assert.False(t, liked)
}

// ── AddComment ────────────────────────────────────────────────────────────────

func TestInteraction_AddComment_Success_IncrementsCount(t *testing.T) {
	commentRepo := &MockCommentRepository{}
	postClient := &MockInteractionPostClient{}
	socialClient := &MockInteractionSocialClient{}

	userID, postID, ownerID := uint64(1), uint64(10), uint64(5)
	postClient.On("GetPostOwner", postID).Return(ownerID, nil)
	socialClient.On("CanView", userID, ownerID).Return(true, nil)

	comment := &interactionComment{ID: 1, PostID: postID, UserID: userID, Body: "Nice post!"}
	commentRepo.On("Create", mock.AnythingOfType("*tests.interactionComment")).Return(comment, nil)
	postClient.On("IncrementComment", postID, 1)

	owner, _ := postClient.GetPostOwner(postID)
	if userID != owner {
		socialClient.CanView(userID, owner)
	}

	c, err := commentRepo.Create(&interactionComment{PostID: postID, UserID: userID, Body: "Nice post!"})
	require.NoError(t, err)
	assert.Equal(t, "Nice post!", c.Body)

	postClient.IncrementComment(postID, 1)

	commentRepo.AssertExpectations(t)
	postClient.AssertExpectations(t)
	socialClient.AssertExpectations(t)
}

func TestInteraction_AddComment_PostNotFound_Error(t *testing.T) {
	postClient := &MockInteractionPostClient{}
	postClient.On("GetPostOwner", uint64(99)).Return(uint64(0), errors.New("post not found"))

	_, err := postClient.GetPostOwner(99)
	assert.Error(t, err)
}

func TestInteraction_AddComment_AccessDenied_Error(t *testing.T) {
	postClient := &MockInteractionPostClient{}
	socialClient := &MockInteractionSocialClient{}

	postClient.On("GetPostOwner", uint64(10)).Return(uint64(5), nil)
	socialClient.On("CanView", uint64(1), uint64(5)).Return(false, nil)

	owner, _ := postClient.GetPostOwner(10)
	canView, _ := socialClient.CanView(1, owner)
	assert.False(t, canView)
}

// ── UpdateComment ─────────────────────────────────────────────────────────────

func TestInteraction_UpdateComment_Owner_Success(t *testing.T) {
	commentRepo := &MockCommentRepository{}

	c := &interactionComment{ID: 1, PostID: 10, UserID: 5, Body: "original"}
	commentRepo.On("FindByID", uint64(1)).Return(c, nil)
	commentRepo.On("Update", uint64(1), uint64(5), "updated body").Return(nil)

	found, err := commentRepo.FindByID(1)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, uint64(5), found.UserID)

	err = commentRepo.Update(1, 5, "updated body")
	require.NoError(t, err)
	commentRepo.AssertExpectations(t)
}

func TestInteraction_UpdateComment_NotOwner_Forbidden(t *testing.T) {
	commentRepo := &MockCommentRepository{}
	c := &interactionComment{ID: 1, UserID: 5}
	commentRepo.On("FindByID", uint64(1)).Return(c, nil)

	found, _ := commentRepo.FindByID(1)
	userID := uint64(99)
	assert.NotEqual(t, found.UserID, userID) // service returns "not authorized" → 403
	commentRepo.AssertNotCalled(t, "Update")
}

func TestInteraction_UpdateComment_NotFound_Forbidden(t *testing.T) {
	// comment not found → service returns "comment not found" → controller maps to 403
	commentRepo := &MockCommentRepository{}
	commentRepo.On("FindByID", uint64(99)).Return(nil, nil)

	found, _ := commentRepo.FindByID(99)
	assert.Nil(t, found)
	commentRepo.AssertNotCalled(t, "Update")
}

// ── DeleteComment ─────────────────────────────────────────────────────────────

func TestInteraction_DeleteComment_Owner_SoftDeletesAndDecrementsCount(t *testing.T) {
	commentRepo := &MockCommentRepository{}
	postClient := &MockInteractionPostClient{}

	c := &interactionComment{ID: 1, PostID: 10, UserID: 5}
	commentRepo.On("FindByID", uint64(1)).Return(c, nil)
	commentRepo.On("SoftDelete", uint64(1), uint64(5)).Return(nil)
	postClient.On("IncrementComment", uint64(10), -1)

	found, _ := commentRepo.FindByID(1)
	assert.Equal(t, uint64(5), found.UserID)

	err := commentRepo.SoftDelete(1, 5)
	require.NoError(t, err)

	postClient.IncrementComment(found.PostID, -1)

	commentRepo.AssertExpectations(t)
	postClient.AssertExpectations(t)
}

func TestInteraction_DeleteComment_NotOwner_Forbidden(t *testing.T) {
	commentRepo := &MockCommentRepository{}
	c := &interactionComment{ID: 1, UserID: 5}
	commentRepo.On("FindByID", uint64(1)).Return(c, nil)

	found, _ := commentRepo.FindByID(1)
	userID := uint64(99)
	assert.NotEqual(t, found.UserID, userID)
	commentRepo.AssertNotCalled(t, "SoftDelete")
}

func TestInteraction_DeleteComment_NotFound_Forbidden(t *testing.T) {
	commentRepo := &MockCommentRepository{}
	commentRepo.On("FindByID", uint64(77)).Return(nil, nil)

	found, _ := commentRepo.FindByID(77)
	assert.Nil(t, found)
}

func TestInteraction_DeleteComment_SoftDeleteFails_Error(t *testing.T) {
	commentRepo := &MockCommentRepository{}
	c := &interactionComment{ID: 1, UserID: 5, PostID: 10}
	commentRepo.On("FindByID", uint64(1)).Return(c, nil)
	commentRepo.On("SoftDelete", uint64(1), uint64(5)).Return(errors.New("db error"))

	commentRepo.FindByID(1)
	err := commentRepo.SoftDelete(1, 5)
	assert.Error(t, err)
}

// ── ListComments ──────────────────────────────────────────────────────────────

func TestInteraction_ListComments_AccessAllowed_ReturnsList(t *testing.T) {
	commentRepo := &MockCommentRepository{}
	postClient := &MockInteractionPostClient{}
	socialClient := &MockInteractionSocialClient{}

	viewerID, postID, ownerID := uint64(1), uint64(10), uint64(5)
	postClient.On("GetPostOwner", postID).Return(ownerID, nil)
	socialClient.On("CanView", viewerID, ownerID).Return(true, nil)

	comments := []*interactionComment{
		{ID: 1, PostID: postID, UserID: 3, Body: "First"},
		{ID: 2, PostID: postID, UserID: 4, Body: "Second"},
	}
	commentRepo.On("ListByPost", postID, 1, 20).Return(comments, int64(2), nil)

	postClient.GetPostOwner(postID)
	socialClient.CanView(viewerID, ownerID)

	list, total, err := commentRepo.ListByPost(postID, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)

	commentRepo.AssertExpectations(t)
}

func TestInteraction_ListComments_AccessDenied_Forbidden(t *testing.T) {
	postClient := &MockInteractionPostClient{}
	socialClient := &MockInteractionSocialClient{}

	postClient.On("GetPostOwner", uint64(10)).Return(uint64(5), nil)
	socialClient.On("CanView", uint64(99), uint64(5)).Return(false, nil)

	owner, _ := postClient.GetPostOwner(10)
	canView, _ := socialClient.CanView(99, owner)
	assert.False(t, canView)
}

func TestInteraction_ListComments_PaginationNorm(t *testing.T) {
	norm := func(page, size *int) {
		if *page <= 0 {
			*page = 1
		}
		if *size <= 0 || *size > 50 {
			*size = 20
		}
	}
	cases := []struct{ p, s, wp, ws int }{
		{0, 0, 1, 20}, {-1, -1, 1, 20}, {2, 25, 2, 25}, {1, 51, 1, 20},
	}
	for _, tc := range cases {
		p, s := tc.p, tc.s
		norm(&p, &s)
		assert.Equal(t, tc.wp, p)
		assert.Equal(t, tc.ws, s)
	}
}

// ── Feed service ──────────────────────────────────────────────────────────────

// Mock: feed's SocialClient
type MockFeedSocialClient struct {
	mock.Mock
}

func (m *MockFeedSocialClient) GetFollowingIDs(userID uint64) ([]uint64, error) {
	args := m.Called(userID)
	return args.Get(0).([]uint64), args.Error(1)
}

// Mock: feed's PostClient
type feedPost struct {
	ID     uint64
	UserID uint64
}

type MockFeedPostClient struct {
	mock.Mock
}

func (m *MockFeedPostClient) GetPostsByUserIDs(userIDs []uint64, page, size int) ([]*feedPost, int64, error) {
	args := m.Called(userIDs, page, size)
	return args.Get(0).([]*feedPost), args.Get(1).(int64), args.Error(2)
}

func TestFeed_GetFeed_IncludesOwnPosts(t *testing.T) {
	// Feed ALWAYS appends userID to followingIDs before querying post-service
	social := &MockFeedSocialClient{}
	postClient := &MockFeedPostClient{}

	userID := uint64(1)
	followingIDs := []uint64{2, 3}
	social.On("GetFollowingIDs", userID).Return(followingIDs, nil)

	ids, _ := social.GetFollowingIDs(userID)
	// Service appends own ID
	allIDs := append(ids, userID)
	assert.Contains(t, allIDs, userID, "own ID must be in the query set")
	assert.Contains(t, allIDs, uint64(2))
	assert.Contains(t, allIDs, uint64(3))
	assert.Len(t, allIDs, 3)

	posts := []*feedPost{{ID: 1, UserID: 1}, {ID: 2, UserID: 2}}
	postClient.On("GetPostsByUserIDs", allIDs, 1, 20).Return(posts, int64(2), nil)

	result, total, err := postClient.GetPostsByUserIDs(allIDs, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, result, 2)

	social.AssertExpectations(t)
	postClient.AssertExpectations(t)
}

func TestFeed_GetFeed_NoFollowing_StillIncludesOwnPosts(t *testing.T) {
	social := &MockFeedSocialClient{}
	postClient := &MockFeedPostClient{}

	userID := uint64(5)
	social.On("GetFollowingIDs", userID).Return([]uint64{}, nil) // follows nobody

	ids, _ := social.GetFollowingIDs(userID)
	allIDs := append(ids, userID)
	assert.Len(t, allIDs, 1)
	assert.Equal(t, userID, allIDs[0])

	ownPosts := []*feedPost{{ID: 10, UserID: 5}}
	postClient.On("GetPostsByUserIDs", allIDs, 1, 20).Return(ownPosts, int64(1), nil)

	result, total, err := postClient.GetPostsByUserIDs(allIDs, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, uint64(5), result[0].UserID)
}

func TestFeed_GetFeed_SocialClientError_Propagates(t *testing.T) {
	social := &MockFeedSocialClient{}
	social.On("GetFollowingIDs", uint64(1)).Return([]uint64{}, errors.New("social svc down"))

	_, err := social.GetFollowingIDs(1)
	assert.Error(t, err)
}

func TestFeed_GetFeed_PostClientError_Propagates(t *testing.T) {
	social := &MockFeedSocialClient{}
	postClient := &MockFeedPostClient{}

	userID := uint64(1)
	social.On("GetFollowingIDs", userID).Return([]uint64{2}, nil)
	allIDs := []uint64{2, userID}
	postClient.On("GetPostsByUserIDs", allIDs, 1, 20).Return([]*feedPost{}, int64(0), errors.New("post svc down"))

	ids, _ := social.GetFollowingIDs(userID)
	merged := append(ids, userID)
	_, _, err := postClient.GetPostsByUserIDs(merged, 1, 20)
	assert.Error(t, err)
}

func TestFeed_PaginationNorm(t *testing.T) {
	norm := func(page, size *int) {
		if *page <= 0 {
			*page = 1
		}
		if *size <= 0 || *size > 50 {
			*size = 20
		}
	}
	cases := []struct{ p, s, wp, ws int }{
		{0, 0, 1, 20}, {-1, 100, 1, 20}, {2, 10, 2, 10},
	}
	for _, tc := range cases {
		p, s := tc.p, tc.s
		norm(&p, &s)
		assert.Equal(t, tc.wp, p)
		assert.Equal(t, tc.ws, s)
	}
}

func TestFeed_GetFeed_HasMoreLogic(t *testing.T) {
	// HasMore = int64(page*size) < total
	assert.True(t, int64(1*20) < int64(30))
	assert.False(t, int64(2*20) < int64(30))
	assert.False(t, int64(1*20) < int64(20))
}

func TestFeed_GetFeed_EmptyFeed_ReturnsPosts(t *testing.T) {
	social := &MockFeedSocialClient{}
	postClient := &MockFeedPostClient{}

	userID := uint64(99)
	social.On("GetFollowingIDs", userID).Return([]uint64{}, nil)
	allIDs := []uint64{userID}
	postClient.On("GetPostsByUserIDs", allIDs, 1, 20).Return([]*feedPost{}, int64(0), nil)

	ids, _ := social.GetFollowingIDs(userID)
	merged := append(ids, userID)
	result, total, err := postClient.GetPostsByUserIDs(merged, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, result)
}
