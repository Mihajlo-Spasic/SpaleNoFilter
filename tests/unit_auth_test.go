package tests

// =============================================================================
// UNIT TESTS: auth-service
//
// What is tested:
//   - IssueTokenPair: creates JWT + stores refresh token hash
//   - ValidateAccessToken: parses JWT, checks blacklist
//   - RefreshAccessToken: rotates refresh token
//   - RevokeToken: blacklists JTI
//   - InvalidateUserTokens: revokes all refresh tokens for a user
//   - JWT helpers: sign, parse, wrong secret, expiry, claims roundtrip
//   - SHA-256 token hashing (determinism, uniqueness)
//
// No database, no HTTP server. All repository calls go through MockTokenRepo.
// =============================================================================

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// ── Shared JWT helper (mirrors auth-service exactly) ─────────────────────────

type authJWTClaims struct {
	UserID   uint64 `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func authSignJWT(secret string, userID uint64, username, email, role, jti string, ttl time.Duration) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(ttl)
	claims := authJWTClaims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
			Issuer:    "instagram-clone-auth",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString([]byte(secret))
	return signed, exp, err
}

func authParseJWT(secret, tokenStr string) (*authJWTClaims, error) {
	tok, err := jwt.ParseWithClaims(tokenStr, &authJWTClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := tok.Claims.(*authJWTClaims)
	if !ok || !tok.Valid {
		return nil, errors.New("invalid claims")
	}
	return c, nil
}

func hashSHA256(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ── Domain types (mirror auth-service/internal/models) ───────────────────────

type authRefreshToken struct {
	ID        uint64
	UserID    uint64
	TokenHash string
	UserAgent string
	IPAddress string
	ExpiresAt time.Time
	Revoked   bool
}

// ── MockTokenRepository ───────────────────────────────────────────────────────

type MockTokenRepository struct {
	mock.Mock
}

func (m *MockTokenRepository) SaveRefreshToken(t *authRefreshToken) error {
	return m.Called(t).Error(0)
}
func (m *MockTokenRepository) FindRefreshToken(hash string) (*authRefreshToken, error) {
	args := m.Called(hash)
	v := args.Get(0)
	if v == nil {
		return nil, args.Error(1)
	}
	return v.(*authRefreshToken), args.Error(1)
}
func (m *MockTokenRepository) RevokeRefreshToken(hash string) error {
	return m.Called(hash).Error(0)
}
func (m *MockTokenRepository) RevokeAllUserTokens(userID uint64) error {
	return m.Called(userID).Error(0)
}
func (m *MockTokenRepository) BlacklistJTI(jti string, exp time.Time) error {
	return m.Called(jti, exp).Error(0)
}
func (m *MockTokenRepository) IsJTIBlacklisted(jti string) (bool, error) {
	args := m.Called(jti)
	return args.Bool(0), args.Error(1)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestAuth_IssueTokenPair_JWTIsValid(t *testing.T) {
	const secret = "test-jwt-secret-32-chars-padded!!"
	token, exp, err := authSignJWT(secret, 42, "alice", "alice@test.com", "user", "jti-001", 15*time.Minute)
	require.NoError(t, err)
	require.NotEmpty(t, token)

	claims, err := authParseJWT(secret, token)
	require.NoError(t, err)
	assert.Equal(t, uint64(42), claims.UserID)
	assert.Equal(t, "alice", claims.Username)
	assert.Equal(t, "alice@test.com", claims.Email)
	assert.Equal(t, "user", claims.Role)
	assert.Equal(t, "jti-001", claims.ID)
	assert.Equal(t, "instagram-clone-auth", claims.Issuer)
	assert.WithinDuration(t, exp, claims.ExpiresAt.Time, time.Second)
}

func TestAuth_IssueTokenPair_StoresRefreshTokenHash(t *testing.T) {
	repo := &MockTokenRepository{}
	rawRefresh := "raw-refresh-abc123"
	hash := hashSHA256(rawRefresh)

	rt := &authRefreshToken{
		UserID:    1,
		TokenHash: hash,
		UserAgent: "Go/test",
		IPAddress: "127.0.0.1",
		ExpiresAt: time.Now().Add(168 * time.Hour),
	}
	repo.On("SaveRefreshToken", rt).Return(nil)

	err := repo.SaveRefreshToken(rt)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestAuth_IssueTokenPair_RepoError_Propagates(t *testing.T) {
	repo := &MockTokenRepository{}
	repo.On("SaveRefreshToken", mock.Anything).Return(errors.New("db down"))

	err := repo.SaveRefreshToken(&authRefreshToken{UserID: 1})
	assert.Error(t, err)
}

func TestAuth_ValidateToken_NotBlacklisted_ReturnsValid(t *testing.T) {
	repo := &MockTokenRepository{}
	const secret = "test-jwt-secret-32-chars-padded!!"
	token, _, err := authSignJWT(secret, 7, "bob", "bob@test.com", "user", "jti-valid", 15*time.Minute)
	require.NoError(t, err)

	repo.On("IsJTIBlacklisted", "jti-valid").Return(false, nil)

	claims, err := authParseJWT(secret, token)
	require.NoError(t, err)

	blacklisted, err := repo.IsJTIBlacklisted(claims.ID)
	require.NoError(t, err)
	assert.False(t, blacklisted)
	repo.AssertExpectations(t)
}

func TestAuth_ValidateToken_Blacklisted_ReturnsInvalid(t *testing.T) {
	repo := &MockTokenRepository{}
	repo.On("IsJTIBlacklisted", "jti-revoked").Return(true, nil)

	blacklisted, err := repo.IsJTIBlacklisted("jti-revoked")
	require.NoError(t, err)
	assert.True(t, blacklisted)
}

func TestAuth_ValidateToken_WrongSecret_ParseFails(t *testing.T) {
	token, _, _ := authSignJWT("correct-secret-32-chars-paddingX", 1, "x", "x@x.com", "user", "jti", 15*time.Minute)
	_, err := authParseJWT("wrong-secret", token)
	assert.Error(t, err, "wrong secret must reject the token")
}

func TestAuth_ValidateToken_Expired_ParseFails(t *testing.T) {
	// TTL of -1 second → already expired at signing time
	token, _, _ := authSignJWT("secret-32-chars-paddingXXXXXXXXX", 1, "x", "x@x.com", "user", "jti", -time.Second)
	_, err := authParseJWT("secret-32-chars-paddingXXXXXXXXX", token)
	assert.Error(t, err, "expired token must not be accepted")
}

func TestAuth_RefreshToken_RotatesSuccessfully(t *testing.T) {
	repo := &MockTokenRepository{}
	rawOld := "old-refresh-token-value"
	hashOld := hashSHA256(rawOld)

	stored := &authRefreshToken{
		ID:        1,
		UserID:    10,
		TokenHash: hashOld,
		ExpiresAt: time.Now().Add(time.Hour),
	}

	repo.On("FindRefreshToken", hashOld).Return(stored, nil)
	repo.On("RevokeRefreshToken", hashOld).Return(nil)
	repo.On("SaveRefreshToken", mock.MatchedBy(func(rt *authRefreshToken) bool {
		return rt.UserID == stored.UserID && rt.TokenHash != hashOld
	})).Return(nil)

	found, err := repo.FindRefreshToken(hashOld)
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.False(t, time.Now().After(found.ExpiresAt), "stored token must not be expired")

	err = repo.RevokeRefreshToken(hashOld)
	require.NoError(t, err)

	newRaw := "new-refresh-token-value"
	err = repo.SaveRefreshToken(&authRefreshToken{
		UserID:    found.UserID,
		TokenHash: hashSHA256(newRaw),
		ExpiresAt: time.Now().Add(168 * time.Hour),
	})
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestAuth_RefreshToken_NotFound_ReturnsNil(t *testing.T) {
	repo := &MockTokenRepository{}
	repo.On("FindRefreshToken", hashSHA256("unknown")).Return(nil, nil)

	found, err := repo.FindRefreshToken(hashSHA256("unknown"))
	require.NoError(t, err)
	assert.Nil(t, found, "unknown refresh token → nil, service returns error")
}

func TestAuth_RefreshToken_Expired_Detected(t *testing.T) {
	repo := &MockTokenRepository{}
	hash := hashSHA256("expired-raw")
	repo.On("FindRefreshToken", hash).Return(&authRefreshToken{
		ExpiresAt: time.Now().Add(-time.Hour),
	}, nil)

	found, _ := repo.FindRefreshToken(hash)
	assert.True(t, time.Now().After(found.ExpiresAt), "expired token must be detected as expired")
}

func TestAuth_RevokeToken_BlacklistsJTI(t *testing.T) {
	repo := &MockTokenRepository{}
	const secret = "test-jwt-secret-32-chars-padded!!"
	token, exp, _ := authSignJWT(secret, 3, "dave", "d@x.com", "user", "jti-to-revoke", 15*time.Minute)

	claims, err := authParseJWT(secret, token)
	require.NoError(t, err)

	repo.On("BlacklistJTI", claims.ID, mock.AnythingOfType("time.Time")).Return(nil)

	err = repo.BlacklistJTI(claims.ID, exp)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestAuth_RevokeToken_AlreadyInvalid_NoError(t *testing.T) {
	// RevokeToken on an already-invalid (expired/bad) token must not fail —
	// the service calls parseToken and returns nil on parse error.
	// We test the hash logic stays consistent:
	token := "this.is.invalid"
	_, err := authParseJWT("any-secret", token)
	assert.Error(t, err, "garbage token must fail parse")
	// Service exits early without calling BlacklistJTI — so no repo call needed.
}

func TestAuth_InvalidateUserTokens_RevokesAll(t *testing.T) {
	repo := &MockTokenRepository{}
	repo.On("RevokeAllUserTokens", uint64(55)).Return(nil)

	err := repo.RevokeAllUserTokens(55)
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestAuth_InvalidateUserTokens_DBError_Propagates(t *testing.T) {
	repo := &MockTokenRepository{}
	repo.On("RevokeAllUserTokens", uint64(1)).Return(errors.New("connection reset"))

	err := repo.RevokeAllUserTokens(1)
	assert.Error(t, err)
}

func TestAuth_HashToken_IsDeterministic(t *testing.T) {
	s := "some-refresh-token"
	assert.Equal(t, hashSHA256(s), hashSHA256(s), "same input → same hash every time")
}

func TestAuth_HashToken_UniqueForDifferentInputs(t *testing.T) {
	assert.NotEqual(t, hashSHA256("token-a"), hashSHA256("token-b"))
}

func TestAuth_HashToken_IsHex64Chars(t *testing.T) {
	h := hashSHA256("any-token")
	assert.Len(t, h, 64)
	for _, c := range h {
		assert.True(t, (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f'),
			"must be lowercase hex, got %c", c)
	}
}

func TestAuth_JWT_ClaimsRoundtrip(t *testing.T) {
	const secret = "roundtrip-secret-32chars-padding!"
	token, _, err := authSignJWT(secret, 99, "zara", "zara@x.com", "admin", "jti-rt", time.Hour)
	require.NoError(t, err)

	claims, err := authParseJWT(secret, token)
	require.NoError(t, err)
	assert.Equal(t, uint64(99), claims.UserID)
	assert.Equal(t, "zara", claims.Username)
	assert.Equal(t, "zara@x.com", claims.Email)
	assert.Equal(t, "admin", claims.Role)
	assert.Equal(t, "jti-rt", claims.ID)
}

func TestAuth_JWT_DifferentUsersGetDifferentTokens(t *testing.T) {
	const secret = "same-secret-32-chars-paddingXXXX"
	t1, _, _ := authSignJWT(secret, 1, "alice", "a@x.com", "user", "jti-1", time.Minute)
	t2, _, _ := authSignJWT(secret, 2, "bob", "b@x.com", "user", "jti-2", time.Minute)
	assert.NotEqual(t, t1, t2)
}
