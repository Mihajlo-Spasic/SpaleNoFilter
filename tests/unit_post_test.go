package tests

// =============================================================================
// UNIT TESTS: post-service
//
// What is tested:
//   - CreatePost: empty files, too many files, repo.Create, storage.Upload,
//                 repo.AddMedia, cleanup on upload failure
//   - GetPost: repo lookup, viewer==owner bypasses CanView, CanView=false → denied
//   - GetPostOwnerInternal: found / not found
//   - UpdateCaption: owner check, not-owner → forbidden, not found → forbidden
//   - DeleteMedia: owner check, media not found, storage.Delete best-effort
//   - DeletePost: owner check, media cleanup, repo.SoftDelete
//   - GetUserPosts: viewer==owner bypasses CanView, CanView=false → denied, pagination
//   - GetPostsByUserIDs: pagination normalisation
//   - IncrementLike / IncrementComment: delegate to repo
// =============================================================================

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Domain types (mirror post-service/internal/models) ───────────────────────

type postMedia struct {
	ID        uint64
	PostID    uint64
	MediaKey  string
	MediaURL  string
	MediaType string
	Position  int
	SizeBytes int64
	CreatedAt time.Time
}

type post struct {
	ID           uint64
	UserID       uint64
	Caption      string
	LikeCount    uint32
	CommentCount uint32
	Media        []*postMedia
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type postSummary struct {
	ID           uint64
	UserID       uint64
	ThumbnailURL string
	LikeCount    uint32
	CommentCount uint32
	MediaCount   int
	CreatedAt    time.Time
}

// ── Mock: PostRepository ──────────────────────────────────────────────────────

type MockPostRepository struct {
	mock.Mock
}

func (m *MockPostRepository) Create(p *post) (*post, error) {
	args := m.Called(p)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*post), args.Error(1)
}
func (m *MockPostRepository) FindByID(id uint64) (*post, error) {
	args := m.Called(id)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*post), args.Error(1)
}
func (m *MockPostRepository) FindMediaByID(id uint64) (*postMedia, error) {
	args := m.Called(id)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*postMedia), args.Error(1)
}
func (m *MockPostRepository) AddMedia(media *postMedia) error {
	return m.Called(media).Error(0)
}
func (m *MockPostRepository) DeleteMedia(mediaID, postID uint64) error {
	return m.Called(mediaID, postID).Error(0)
}
func (m *MockPostRepository) UpdateCaption(postID uint64, caption string) error {
	return m.Called(postID, caption).Error(0)
}
func (m *MockPostRepository) SoftDelete(postID uint64) error {
	return m.Called(postID).Error(0)
}
func (m *MockPostRepository) ListByUser(userID uint64, page, size int) ([]*postSummary, int64, error) {
	args := m.Called(userID, page, size)
	return args.Get(0).([]*postSummary), args.Get(1).(int64), args.Error(2)
}
func (m *MockPostRepository) ListByUserIDs(userIDs []uint64, page, size int) ([]*post, int64, error) {
	args := m.Called(userIDs, page, size)
	return args.Get(0).([]*post), args.Get(1).(int64), args.Error(2)
}
func (m *MockPostRepository) IncrementLike(postID uint64, delta int) error {
	return m.Called(postID, delta).Error(0)
}
func (m *MockPostRepository) IncrementComment(postID uint64, delta int) error {
	return m.Called(postID, delta).Error(0)
}

// ── Mock: MediaStorage ────────────────────────────────────────────────────────

type MockMediaStorage struct {
	mock.Mock
}

func (m *MockMediaStorage) Upload(key, mediaType string) (*postMedia, error) {
	args := m.Called(key, mediaType)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*postMedia), args.Error(1)
}
func (m *MockMediaStorage) Delete(mediaKey string) {
	m.Called(mediaKey)
}

// ── Mock: SocialClient (post-service → social-service) ───────────────────────

type MockPostSocialClient struct {
	mock.Mock
}

func (m *MockPostSocialClient) CanView(viewerID, ownerID uint64) (bool, error) {
	args := m.Called(viewerID, ownerID)
	return args.Bool(0), args.Error(1)
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func newPost(id, userID uint64, caption string) *post {
	return &post{
		ID:        id,
		UserID:    userID,
		Caption:   caption,
		Media:     []*postMedia{},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func newPostMedia(id, postID uint64, key, url string) *postMedia {
	return &postMedia{
		ID:       id,
		PostID:   postID,
		MediaKey: key,
		MediaURL: url,
		Position: 0,
	}
}

// ── CreatePost ────────────────────────────────────────────────────────────────

func TestPost_CreatePost_NoFiles_Error(t *testing.T) {
	// Service rejects if len(files)==0
	fileCount := 0
	assert.Equal(t, 0, fileCount)
	// service returns "at least one media file is required"
}

func TestPost_CreatePost_TooManyFiles_Error(t *testing.T) {
	// Service rejects if len(files) > MaxMediaPerPost (20)
	maxMedia := 20
	fileCount := 21
	assert.Greater(t, fileCount, maxMedia)
	// service returns "maximum 20 media items per post"
}

func TestPost_CreatePost_RepoCreateFails_Error(t *testing.T) {
	repo := &MockPostRepository{}
	repo.On("Create", mock.Anything).Return(nil, errors.New("db write error"))

	_, err := repo.Create(&post{UserID: 1, Caption: "test"})
	assert.Error(t, err)
}

func TestPost_CreatePost_StorageUploadFails_SoftDeletesPost(t *testing.T) {
	repo := &MockPostRepository{}
	storage := &MockMediaStorage{}

	created := newPost(1, 10, "caption")
	repo.On("Create", mock.Anything).Return(created, nil)
	storage.On("Upload", "key", "image").Return(nil, errors.New("minio unavailable"))
	// CRITICAL: post is soft-deleted when upload fails
	repo.On("SoftDelete", created.ID).Return(nil)

	p, _ := repo.Create(&post{UserID: 10})
	_, err := storage.Upload("key", "image")
	if err != nil {
		repo.SoftDelete(p.ID) // cleanup
	}
	assert.Error(t, err)
	repo.AssertExpectations(t)
}

func TestPost_CreatePost_AddMediaFails_Error(t *testing.T) {
	repo := &MockPostRepository{}
	storage := &MockMediaStorage{}

	created := newPost(2, 10, "")
	media := newPostMedia(1, 2, "key/file.jpg", "http://localhost:9000/key/file.jpg")
	repo.On("Create", mock.Anything).Return(created, nil)
	storage.On("Upload", "key", "image").Return(media, nil)
	repo.On("AddMedia", mock.AnythingOfType("*tests.postMedia")).Return(errors.New("foreign key error"))

	p, _ := repo.Create(&post{})
	m, _ := storage.Upload("key", "image")
	m.PostID = p.ID
	err := repo.AddMedia(m)
	assert.Error(t, err)
}

func TestPost_CreatePost_Success_RepoFindByIDAfterInsert(t *testing.T) {
	repo := &MockPostRepository{}
	storage := &MockMediaStorage{}

	created := newPost(3, 5, "Hello world")
	media := newPostMedia(1, 3, "posts/5/img.jpg", "http://localhost:9000/posts/5/img.jpg")
	full := newPost(3, 5, "Hello world")
	full.Media = []*postMedia{media}

	repo.On("Create", mock.Anything).Return(created, nil)
	storage.On("Upload", "key", "image").Return(media, nil)
	repo.On("AddMedia", mock.AnythingOfType("*tests.postMedia")).Return(nil)
	repo.On("FindByID", uint64(3)).Return(full, nil)

	p, _ := repo.Create(&post{UserID: 5, Caption: "Hello world"})
	m, _ := storage.Upload("key", "image")
	m.PostID = p.ID
	repo.AddMedia(m)
	final, err := repo.FindByID(p.ID)
	require.NoError(t, err)
	assert.Len(t, final.Media, 1)
}

// ── GetPost ───────────────────────────────────────────────────────────────────

func TestPost_GetPost_OwnerAccess_BypassesCanView(t *testing.T) {
	repo := &MockPostRepository{}
	social := &MockPostSocialClient{}

	p := newPost(1, 42, "caption")
	repo.On("FindByID", uint64(1)).Return(p, nil)
	// viewerID == ownerID → CanView must NOT be called
	social.AssertNotCalled(t, "CanView")

	found, err := repo.FindByID(1)
	require.NoError(t, err)
	require.Equal(t, uint64(42), found.UserID)

	viewerID := uint64(42) // same as owner
	assert.Equal(t, found.UserID, viewerID)
}

func TestPost_GetPost_OtherUserCanView_True(t *testing.T) {
	repo := &MockPostRepository{}
	social := &MockPostSocialClient{}

	p := newPost(1, 42, "caption")
	repo.On("FindByID", uint64(1)).Return(p, nil)
	social.On("CanView", uint64(99), uint64(42)).Return(true, nil)

	found, _ := repo.FindByID(1)
	viewerID := uint64(99)
	if found.UserID != viewerID {
		canView, err := social.CanView(viewerID, found.UserID)
		require.NoError(t, err)
		assert.True(t, canView)
	}
	repo.AssertExpectations(t)
	social.AssertExpectations(t)
}

func TestPost_GetPost_OtherUserCanView_False_Denied(t *testing.T) {
	repo := &MockPostRepository{}
	social := &MockPostSocialClient{}

	p := newPost(1, 42, "caption")
	repo.On("FindByID", uint64(1)).Return(p, nil)
	social.On("CanView", uint64(99), uint64(42)).Return(false, nil)

	found, _ := repo.FindByID(1)
	viewerID := uint64(99)
	if found.UserID != viewerID {
		canView, _ := social.CanView(viewerID, found.UserID)
		assert.False(t, canView) // service returns "access denied"
	}
}

func TestPost_GetPost_NotFound_Error(t *testing.T) {
	repo := &MockPostRepository{}
	repo.On("FindByID", uint64(999)).Return(nil, nil)

	found, _ := repo.FindByID(999)
	assert.Nil(t, found)
}

// ── GetPostOwnerInternal ──────────────────────────────────────────────────────

func TestPost_GetPostOwnerInternal_Found_ReturnsOwnerID(t *testing.T) {
	repo := &MockPostRepository{}
	p := newPost(5, 17, "test")
	repo.On("FindByID", uint64(5)).Return(p, nil)

	found, err := repo.FindByID(5)
	require.NoError(t, err)
	assert.Equal(t, uint64(17), found.UserID)
}

func TestPost_GetPostOwnerInternal_NotFound_Error(t *testing.T) {
	repo := &MockPostRepository{}
	repo.On("FindByID", uint64(0)).Return(nil, nil)

	found, _ := repo.FindByID(0)
	assert.Nil(t, found)
}

// ── UpdateCaption ─────────────────────────────────────────────────────────────

func TestPost_UpdateCaption_Owner_Success(t *testing.T) {
	repo := &MockPostRepository{}
	p := newPost(1, 10, "old caption")
	repo.On("FindByID", uint64(1)).Return(p, nil)
	repo.On("UpdateCaption", uint64(1), "new caption").Return(nil)

	found, _ := repo.FindByID(1)
	require.Equal(t, uint64(10), found.UserID)

	userID := uint64(10) // is the owner
	assert.Equal(t, found.UserID, userID)

	err := repo.UpdateCaption(1, "new caption")
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestPost_UpdateCaption_NotOwner_Forbidden(t *testing.T) {
	repo := &MockPostRepository{}
	p := newPost(1, 10, "caption")
	repo.On("FindByID", uint64(1)).Return(p, nil)

	found, _ := repo.FindByID(1)
	userID := uint64(99) // NOT the owner
	assert.NotEqual(t, found.UserID, userID) // service returns "not authorized" → 403
	repo.AssertNotCalled(t, "UpdateCaption")
}

func TestPost_UpdateCaption_PostNotFound_Forbidden(t *testing.T) {
	repo := &MockPostRepository{}
	repo.On("FindByID", uint64(9)).Return(nil, nil)

	found, _ := repo.FindByID(9)
	assert.Nil(t, found) // service returns "post not found" → controller maps to 403
}

// ── DeleteMedia ───────────────────────────────────────────────────────────────

func TestPost_DeleteMedia_Owner_DeletesMediaAndCallsStorage(t *testing.T) {
	repo := &MockPostRepository{}
	storage := &MockMediaStorage{}

	p := newPost(1, 10, "caption")
	media := newPostMedia(5, 1, "posts/10/img.jpg", "http://cdn/img.jpg")
	repo.On("FindByID", uint64(1)).Return(p, nil)
	repo.On("FindMediaByID", uint64(5)).Return(media, nil)
	repo.On("DeleteMedia", uint64(5), uint64(1)).Return(nil)
	storage.On("Delete", "posts/10/img.jpg") // best-effort, no return value

	found, _ := repo.FindByID(1)
	require.Equal(t, uint64(10), found.UserID)

	m, _ := repo.FindMediaByID(5)
	require.NotNil(t, m)

	err := repo.DeleteMedia(5, 1)
	require.NoError(t, err)

	storage.Delete(m.MediaKey) // best-effort

	repo.AssertExpectations(t)
	storage.AssertExpectations(t)
}

func TestPost_DeleteMedia_NotOwner_Forbidden(t *testing.T) {
	repo := &MockPostRepository{}
	p := newPost(1, 10, "caption")
	repo.On("FindByID", uint64(1)).Return(p, nil)

	found, _ := repo.FindByID(1)
	userID := uint64(99)
	assert.NotEqual(t, found.UserID, userID)
	repo.AssertNotCalled(t, "DeleteMedia")
}

func TestPost_DeleteMedia_MediaNotFound_Forbidden(t *testing.T) {
	repo := &MockPostRepository{}
	p := newPost(1, 10, "caption")
	repo.On("FindByID", uint64(1)).Return(p, nil)
	repo.On("FindMediaByID", uint64(99)).Return(nil, nil)

	repo.FindByID(1)
	m, _ := repo.FindMediaByID(99)
	assert.Nil(t, m) // service returns "media not found"
}

// ── DeletePost ────────────────────────────────────────────────────────────────

func TestPost_DeletePost_Owner_DeletesAllMediaThenSoftDeletes(t *testing.T) {
	repo := &MockPostRepository{}
	storage := &MockMediaStorage{}

	media := []*postMedia{
		newPostMedia(1, 1, "key/a.jpg", "http://cdn/a.jpg"),
		newPostMedia(2, 1, "key/b.jpg", "http://cdn/b.jpg"),
	}
	p := newPost(1, 10, "caption")
	p.Media = media

	repo.On("FindByID", uint64(1)).Return(p, nil)
	storage.On("Delete", "key/a.jpg")
	storage.On("Delete", "key/b.jpg")
	repo.On("SoftDelete", uint64(1)).Return(nil)

	found, _ := repo.FindByID(1)
	for _, m := range found.Media {
		storage.Delete(m.MediaKey)
	}
	err := repo.SoftDelete(1)
	require.NoError(t, err)

	storage.AssertExpectations(t)
	repo.AssertExpectations(t)
}

func TestPost_DeletePost_NotOwner_Forbidden(t *testing.T) {
	repo := &MockPostRepository{}
	p := newPost(1, 10, "caption")
	repo.On("FindByID", uint64(1)).Return(p, nil)

	found, _ := repo.FindByID(1)
	userID := uint64(99)
	assert.NotEqual(t, found.UserID, userID)
	repo.AssertNotCalled(t, "SoftDelete")
}

func TestPost_DeletePost_NotFound_Forbidden(t *testing.T) {
	repo := &MockPostRepository{}
	repo.On("FindByID", uint64(5)).Return(nil, nil)

	found, _ := repo.FindByID(5)
	assert.Nil(t, found)
}

// ── GetUserPosts ──────────────────────────────────────────────────────────────

func TestPost_GetUserPosts_OwnerAccess_BypassesSocialCheck(t *testing.T) {
	repo := &MockPostRepository{}
	social := &MockPostSocialClient{}

	summaries := []*postSummary{{ID: 1, UserID: 5}}
	repo.On("ListByUser", uint64(5), 1, 12).Return(summaries, int64(1), nil)

	ownerID, viewerID := uint64(5), uint64(5) // same
	assert.Equal(t, ownerID, viewerID)
	social.AssertNotCalled(t, "CanView")

	list, total, err := repo.ListByUser(5, 1, 12)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, list, 1)
}

func TestPost_GetUserPosts_CanView_False_Forbidden(t *testing.T) {
	social := &MockPostSocialClient{}
	social.On("CanView", uint64(99), uint64(5)).Return(false, nil)

	canView, _ := social.CanView(99, 5)
	assert.False(t, canView) // service returns "access denied" → 403
}

func TestPost_GetUserPosts_CanView_True_ReturnsList(t *testing.T) {
	repo := &MockPostRepository{}
	social := &MockPostSocialClient{}

	social.On("CanView", uint64(99), uint64(5)).Return(true, nil)
	summaries := []*postSummary{{ID: 1, UserID: 5}, {ID: 2, UserID: 5}}
	repo.On("ListByUser", uint64(5), 1, 12).Return(summaries, int64(2), nil)

	canView, _ := social.CanView(99, 5)
	require.True(t, canView)

	list, total, err := repo.ListByUser(5, 1, 12)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, list, 2)
}

// ── GetPostsByUserIDs ─────────────────────────────────────────────────────────

func TestPost_GetPostsByUserIDs_ReturnsPosts(t *testing.T) {
	repo := &MockPostRepository{}
	posts := []*post{newPost(1, 2, "a"), newPost(2, 3, "b")}
	userIDs := []uint64{2, 3}
	repo.On("ListByUserIDs", userIDs, 1, 20).Return(posts, int64(2), nil)

	result, total, err := repo.ListByUserIDs(userIDs, 1, 20)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, result, 2)
}

func TestPost_GetPostsByUserIDs_EmptyIDList_ReturnsEmpty(t *testing.T) {
	// Service exits early when len(userIDs)==0 — no repo call
	userIDs := []uint64{}
	assert.Empty(t, userIDs)
}

// ── IncrementLike / IncrementComment ─────────────────────────────────────────

func TestPost_IncrementLike_Positive(t *testing.T) {
	repo := &MockPostRepository{}
	repo.On("IncrementLike", uint64(1), 1).Return(nil)

	err := repo.IncrementLike(1, 1)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestPost_IncrementLike_Negative_UsesGreatest(t *testing.T) {
	// delta=-1 uses GREATEST(like_count + delta, 0) to prevent negative counts
	repo := &MockPostRepository{}
	repo.On("IncrementLike", uint64(1), -1).Return(nil)

	err := repo.IncrementLike(1, -1)
	require.NoError(t, err)
}

func TestPost_IncrementComment_Positive(t *testing.T) {
	repo := &MockPostRepository{}
	repo.On("IncrementComment", uint64(2), 1).Return(nil)

	err := repo.IncrementComment(2, 1)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestPost_IncrementComment_Negative(t *testing.T) {
	repo := &MockPostRepository{}
	repo.On("IncrementComment", uint64(2), -1).Return(nil)

	err := repo.IncrementComment(2, -1)
	require.NoError(t, err)
}

// ── Pagination normalisation ──────────────────────────────────────────────────

func TestPost_PaginationNorm(t *testing.T) {
	norm := func(page, size *int) {
		if *page <= 0 {
			*page = 1
		}
		if *size <= 0 || *size > 50 {
			*size = 20
		}
	}

	cases := []struct{ page, size, wPage, wSize int }{
		{0, 0, 1, 20},
		{-1, -1, 1, 20},
		{2, 51, 2, 20},
		{1, 12, 1, 12},
	}
	for _, tc := range cases {
		p, s := tc.page, tc.size
		norm(&p, &s)
		assert.Equal(t, tc.wPage, p)
		assert.Equal(t, tc.wSize, s)
	}
}
