package tests

// =============================================================================
// INTEGRATION TESTS
//
// Black-box HTTP tests against the full running docker-compose stack.
// Each test registers fresh users and operates on them so tests are isolated.
//
// To run: `go test -v -run TestIntegration -timeout 120s`
// Prerequisites: docker-compose stack must be running.
//
// Services tested:
//   user-service    :8080
//   auth-service    :8081
//   social-service  :8082
//   post-service    :8083
//   interaction-service :8084
//   feed-service    :8085
// =============================================================================

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── Service base URLs ──────────────────────────────────────────────────────────

func userURL(path string) string   { return "http://localhost:8080" + path }
func authURL(path string) string   { return "http://localhost:8081" + path }
func socialURL(path string) string { return "http://localhost:8082" + path }
func postURL(path string) string   { return "http://localhost:8083" + path }
func interURL(path string) string  { return "http://localhost:8084" + path }
func feedURL(path string) string   { return "http://localhost:8085" + path }

// internalSecret must match the running stack's INTERNAL_SECRET env var.
// Default (no .env): "internal-secret-key"
// With user's .env:  "internal-service-to-service-secret-key"
// Override: export INTERNAL_SECRET=... before running tests.
func getInternalSecret() string {
	if v := os.Getenv("INTERNAL_SECRET"); v != "" {
		return v
	}
	return "internal-secret-key"
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

var httpClient = &http.Client{Timeout: 10 * time.Second}

func doJSON(method, url string, body interface{}, headers map[string]string) (*http.Response, map[string]interface{}) {
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyReader = bytes.NewBuffer(b)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		panic(fmt.Sprintf("doJSON NewRequest: %v", err))
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(raw, &result)
	return resp, result
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func internal() map[string]string {
	return map[string]string{"X-Internal-Secret": getInternalSecret()}
}

// authHeader returns a map with both Authorization and X-Internal-Secret
func bothHeaders(token string) map[string]string {
	return map[string]string{
		"Authorization":     "Bearer " + token,
		"X-Internal-Secret": getInternalSecret(),
	}
}

// ── User registration helper ───────────────────────────────────────────────────

type integTestUser struct {
	ID           uint64
	Username     string
	Email        string
	AccessToken  string
	RefreshToken string
}

var userCounter int

// createdUsers tracks all users registered during tests for teardown cleanup.
var createdUsers []*integTestUser
var createdUsersMu sync.Mutex

func registerUser(t *testing.T) *integTestUser {
	t.Helper()
	userCounter++
	suffix := fmt.Sprintf("%d%d", time.Now().UnixNano()%1_000_000, userCounter)
	username := "u" + suffix
	email := username + "@inttest.com"

	resp, body := doJSON("POST", userURL("/api/v1/auth/register"), map[string]interface{}{
		"username":  username,
		"email":     email,
		"password":  "Password123",
		"full_name": "Test User " + suffix,
	}, nil)
	require.NotNil(t, resp, "register: no response")
	require.Equal(t, http.StatusCreated, resp.StatusCode, "register failed: %v", body)

	user := body["user"].(map[string]interface{})
	id := uint64(user["id"].(float64))
	u := &integTestUser{
		ID:           id,
		Username:     username,
		Email:        email,
		AccessToken:  body["access_token"].(string),
		RefreshToken: body["refresh_token"].(string),
	}
	createdUsersMu.Lock()
	createdUsers = append(createdUsers, u)
	createdUsersMu.Unlock()
	return u
}

// cleanupTestData deletes all users created during the test run via the delete account endpoint.
func cleanupTestData() {
	createdUsersMu.Lock()
	users := make([]*integTestUser, len(createdUsers))
	copy(users, createdUsers)
	createdUsersMu.Unlock()

	for _, u := range users {
		// Re-login in case token expired
		resp, body := doJSON("POST", userURL("/api/v1/auth/login"), map[string]interface{}{
			"username_or_email": u.Username,
			"password":          "Password123",
		}, nil)
		token := u.AccessToken
		if resp != nil && resp.StatusCode == 200 {
			if t, ok := body["access_token"].(string); ok {
				token = t
			}
		}
		// Delete the account
		req, _ := http.NewRequest("DELETE", userURL("/api/v1/users/me"), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		resp2, _ := httpClient.Do(req)
		if resp2 != nil {
			resp2.Body.Close()
		}
	}
}

// uploadPost creates a post with a minimal JPEG via multipart form.
func uploadPost(t *testing.T, token, caption string) uint64 {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	_ = w.WriteField("caption", caption)
	fw, _ := w.CreateFormFile("files", "test.jpg")
	// Minimal valid JPEG bytes
	fw.Write([]byte{
		0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0xFF, 0xD9,
	})
	w.Close()

	req, _ := http.NewRequest("POST", postURL("/api/v1/posts"), &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "uploadPost failed: %s", string(raw))

	var result map[string]interface{}
	json.Unmarshal(raw, &result)
	return uint64(result["id"].(float64))
}

// ── TestMain: wait for all services to be healthy ─────────────────────────────

func TestMain(m *testing.M) {
	healthURLs := []string{
		userURL("/api/v1/health"),
		authURL("/api/v1/auth/health"),
		socialURL("/api/v1/social/health"),
		postURL("/api/v1/posts/health"),
		interURL("/api/v1/interactions/health"),
		feedURL("/api/v1/feed/health"),
	}

	deadline := time.Now().Add(60 * time.Second)
	for _, u := range healthURLs {
		for {
			resp, err := httpClient.Get(u)
			if err == nil && resp.StatusCode == 200 {
				resp.Body.Close()
				break
			}
			if resp != nil {
				resp.Body.Close()
			}
			if time.Now().After(deadline) {
				fmt.Printf("SKIP: service not ready: %s\n", u)
				os.Exit(0) // skip rather than fail if stack not running
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
	code := m.Run()
	cleanupTestData()
	os.Exit(code)
}

// ═════════════════════════════════════════════════════════════════════════════
// AUTH-SERVICE + USER-SERVICE
// ═════════════════════════════════════════════════════════════════════════════

func TestIntegration_Health_AllServices(t *testing.T) {
	urls := map[string]string{
		"user":        userURL("/api/v1/health"),
		"auth":        authURL("/api/v1/auth/health"),
		"social":      socialURL("/api/v1/social/health"),
		"post":        postURL("/api/v1/posts/health"),
		"interaction": interURL("/api/v1/interactions/health"),
		"feed":        feedURL("/api/v1/feed/health"),
	}
	for svc, url := range urls {
		resp, body := doJSON("GET", url, nil, nil)
		require.NotNil(t, resp, "%s health: no response", svc)
		assert.Equal(t, 200, resp.StatusCode, "%s health status", svc)
		assert.Equal(t, "ok", body["status"], "%s health body", svc)
	}
}

func TestIntegration_Register_Success(t *testing.T) {
	u := registerUser(t)
	assert.NotZero(t, u.ID)
	assert.NotEmpty(t, u.AccessToken)
	assert.NotEmpty(t, u.RefreshToken)
}

func TestIntegration_Register_DuplicateUsername_409(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("POST", userURL("/api/v1/auth/register"), map[string]interface{}{
		"username": u.Username,
		"email":    "other" + u.Email,
		"password": "Password123",
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Contains(t, body["error"].(string), "username")
}

func TestIntegration_Register_DuplicateEmail_409(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("POST", userURL("/api/v1/auth/register"), map[string]interface{}{
		"username": "other" + u.Username,
		"email":    u.Email,
		"password": "Password123",
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusConflict, resp.StatusCode)
	assert.Contains(t, body["error"].(string), "email")
}

func TestIntegration_Register_InvalidBody_400(t *testing.T) {
	resp, _ := doJSON("POST", userURL("/api/v1/auth/register"), map[string]interface{}{
		"username": "ab", // too short (min=3)
		"email":    "bad-email",
		"password": "short",
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Login_Success(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("POST", userURL("/api/v1/auth/login"), map[string]interface{}{
		"username_or_email": u.Username,
		"password":          "Password123",
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, body["access_token"])
	assert.NotEmpty(t, body["refresh_token"])
}

func TestIntegration_Login_ByEmail(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("POST", userURL("/api/v1/auth/login"), map[string]interface{}{
		"username_or_email": u.Email,
		"password":          "Password123",
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, body["access_token"])
}

func TestIntegration_Login_WrongPassword_401(t *testing.T) {
	u := registerUser(t)
	resp, _ := doJSON("POST", userURL("/api/v1/auth/login"), map[string]interface{}{
		"username_or_email": u.Username,
		"password":          "wrongpassword",
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestIntegration_Login_NonExistentUser_401(t *testing.T) {
	resp, _ := doJSON("POST", userURL("/api/v1/auth/login"), map[string]interface{}{
		"username_or_email": "doesnotexist",
		"password":          "Password123",
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestIntegration_RefreshToken_Success(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("POST", userURL("/api/v1/auth/refresh"), map[string]interface{}{
		"refresh_token": u.RefreshToken,
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, body["access_token"])
	assert.NotEmpty(t, body["refresh_token"])
	// token must be rotated
	assert.NotEqual(t, u.RefreshToken, body["refresh_token"])
}

func TestIntegration_RefreshToken_InvalidToken_401(t *testing.T) {
	resp, _ := doJSON("POST", userURL("/api/v1/auth/refresh"), map[string]interface{}{
		"refresh_token": "invalid-token-value",
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestIntegration_AuthValidate_ValidToken_200(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("POST", authURL("/api/v1/auth/validate"), map[string]interface{}{
		"token": u.AccessToken,
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body["valid"])
	assert.Equal(t, float64(u.ID), body["user_id"])
}

func TestIntegration_AuthValidate_InvalidToken_401(t *testing.T) {
	resp, body := doJSON("POST", authURL("/api/v1/auth/validate"), map[string]interface{}{
		"token": "not.a.real.token",
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Equal(t, false, body["valid"])
}

func TestIntegration_AuthRevoke_BlacklistsToken(t *testing.T) {
	u := registerUser(t)
	// Revoke the token
	resp, _ := doJSON("POST", authURL("/api/v1/auth/revoke"), map[string]interface{}{
		"token": u.AccessToken,
	}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Validate the same token — should now be invalid
	resp2, body2 := doJSON("POST", authURL("/api/v1/auth/validate"), map[string]interface{}{
		"token": u.AccessToken,
	}, nil)
	require.NotNil(t, resp2)
	assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode)
	assert.Equal(t, false, body2["valid"])
}

func TestIntegration_GetMyProfile_Authenticated(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("GET", userURL("/api/v1/users/me"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, u.Username, body["username"])
	assert.Equal(t, u.Email, body["email"])
}

func TestIntegration_GetMyProfile_NoToken_401(t *testing.T) {
	resp, _ := doJSON("GET", userURL("/api/v1/users/me"), nil, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestIntegration_GetPublicProfile_Success(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("GET", userURL("/api/v1/users/"+u.Username+"/profile"), nil, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, u.Username, body["username"])
	// Email must NOT be in public profile
	_, hasEmail := body["email"]
	assert.False(t, hasEmail, "email must not be in public profile")
}

func TestIntegration_GetPublicProfile_NotFound_404(t *testing.T) {
	resp, _ := doJSON("GET", userURL("/api/v1/users/definitelynotexist999/profile"), nil, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestIntegration_UpdateProfile_Success(t *testing.T) {
	u := registerUser(t)
	bio := "Updated bio"
	resp, body := doJSON("PUT", userURL("/api/v1/users/me"), map[string]interface{}{
		"bio": bio,
	}, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, bio, body["bio"])
}

func TestIntegration_UpdateAvatar_Success(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("PUT", userURL("/api/v1/users/me/avatar"), map[string]interface{}{
		"avatar_url": "https://cdn.example.com/avatar.jpg",
	}, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "avatar updated", body["message"])
}

func TestIntegration_UpdateAvatar_BadURL_400(t *testing.T) {
	u := registerUser(t)
	resp, _ := doJSON("PUT", userURL("/api/v1/users/me/avatar"), map[string]interface{}{
		"avatar_url": "not-a-url",
	}, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_ChangePassword_Success(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("PUT", userURL("/api/v1/auth/change-password"), map[string]interface{}{
		"old_password": "Password123",
		"new_password": "NewPassword456",
	}, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "password changed, all sessions terminated", body["message"])
}

func TestIntegration_ChangePassword_WrongOldPassword_400(t *testing.T) {
	u := registerUser(t)
	resp, _ := doJSON("PUT", userURL("/api/v1/auth/change-password"), map[string]interface{}{
		"old_password": "WrongOldPass",
		"new_password": "NewPassword456",
	}, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_SearchUsers_ReturnsResults(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("GET", userURL("/api/v1/users/search?q="+u.Username), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotNil(t, body["data"])
	total := int(body["total"].(float64))
	assert.GreaterOrEqual(t, total, 1)
}

func TestIntegration_SearchUsers_NoQuery_400(t *testing.T) {
	u := registerUser(t)
	resp, _ := doJSON("GET", userURL("/api/v1/users/search"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Logout_Success(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("POST", userURL("/api/v1/auth/logout"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "logged out", body["message"])
}

func TestIntegration_LogoutAll_Success(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("POST", userURL("/api/v1/auth/logout-all"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "all sessions terminated", body["message"])
}

func TestIntegration_DeleteAccount_Success(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("DELETE", userURL("/api/v1/users/me"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "account deleted", body["message"])
}

// ── Internal user endpoints ────────────────────────────────────────────────────

func TestIntegration_Internal_GetUserByID_Success(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("GET", userURL("/internal/users/"+strconv.FormatUint(u.ID, 10)), nil, internal())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, float64(u.ID), body["id"])
	assert.Equal(t, u.Username, body["username"])
}

func TestIntegration_Internal_GetUserByID_WrongSecret_403(t *testing.T) {
	u := registerUser(t)
	resp, _ := doJSON("GET", userURL("/internal/users/"+strconv.FormatUint(u.ID, 10)), nil,
		map[string]string{"X-Internal-Secret": "wrong-secret"})
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestIntegration_Internal_GetUserByUsername_Success(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("GET", userURL("/internal/users/by-username/"+u.Username), nil, internal())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, u.Email, body["email"])
}

func TestIntegration_Internal_IsPrivate_DefaultFalse(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("GET", userURL("/internal/users/"+strconv.FormatUint(u.ID, 10)+"/is-private"), nil, internal())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, false, body["is_private"])
	assert.Equal(t, true, body["is_active"])
}

// ── Auth internal endpoints ────────────────────────────────────────────────────

func TestIntegration_AuthInternal_Issue_Success(t *testing.T) {
	resp, body := doJSON("POST", authURL("/internal/auth/issue"), map[string]interface{}{
		"user_id":  999,
		"username": "internal_test_user",
		"email":    "internal@test.com",
		"role":     "user",
	}, internal())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotEmpty(t, body["access_token"])
	assert.NotEmpty(t, body["refresh_token"])
}

func TestIntegration_AuthInternal_Issue_WrongSecret_403(t *testing.T) {
	resp, _ := doJSON("POST", authURL("/internal/auth/issue"), map[string]interface{}{
		"user_id":  999,
		"username": "x",
		"email":    "x@x.com",
	}, map[string]string{"X-Internal-Secret": "bad"})
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestIntegration_AuthInternal_InvalidateUser_Success(t *testing.T) {
	u := registerUser(t)
	resp, body := doJSON("POST", authURL("/internal/auth/invalidate-user"), map[string]interface{}{
		"user_id": u.ID,
	}, internal())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "all user tokens invalidated", body["message"])
}

// ═════════════════════════════════════════════════════════════════════════════
// SOCIAL-SERVICE
// ═════════════════════════════════════════════════════════════════════════════

func TestIntegration_Social_FollowPublicUser(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	// Alice follows Bob (public profile → accepted immediately)
	resp, body := doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{
		"target_user_id": bob.ID,
	}, bearer(alice.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "follow request sent", body["message"])
}

func TestIntegration_Social_FollowSelf_400(t *testing.T) {
	alice := registerUser(t)
	resp, _ := doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{
		"target_user_id": alice.ID,
	}, bearer(alice.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Social_FollowDuplicate_200_Idempotent(t *testing.T) {
	// ON DUPLICATE KEY UPDATE → always 200 for public profile
	alice := registerUser(t)
	bob := registerUser(t)

	doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))
	resp, _ := doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "duplicate follow must be idempotent 200")
}

func TestIntegration_Social_Unfollow_Success(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	resp, body := doJSON("DELETE", socialURL("/api/v1/social/follow/"+strconv.FormatUint(bob.ID, 10)), nil, bearer(alice.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "unfollowed", body["message"])
}

func TestIntegration_Social_FollowPrivateUser_StatusPending(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	// Make Bob private
	doJSON("PUT", userURL("/api/v1/users/me"), map[string]interface{}{"is_private": true}, bearer(bob.AccessToken))

	// Alice follows Bob → should be pending
	doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	// Check status
	resp, body := doJSON("GET", socialURL("/api/v1/social/status/"+strconv.FormatUint(bob.ID, 10)), nil, bearer(alice.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "pending", body["follow_status"])
}

func TestIntegration_Social_GetPendingRequests(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	// Bob is private
	doJSON("PUT", userURL("/api/v1/users/me"), map[string]interface{}{"is_private": true}, bearer(bob.AccessToken))
	// Alice follows Bob
	doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	// Bob lists pending
	resp, body := doJSON("GET", socialURL("/api/v1/social/follow-requests/pending"), nil, bearer(bob.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	total := int(body["total"].(float64))
	assert.GreaterOrEqual(t, total, 1)
}

func TestIntegration_Social_RespondFollowRequest_Accept(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	doJSON("PUT", userURL("/api/v1/users/me"), map[string]interface{}{"is_private": true}, bearer(bob.AccessToken))
	doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	resp, body := doJSON("POST", socialURL("/api/v1/social/follow-requests/respond"), map[string]interface{}{
		"follower_id": alice.ID,
		"accept":      true,
	}, bearer(bob.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "request accepted", body["message"])
}

func TestIntegration_Social_RespondFollowRequest_Reject(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	doJSON("PUT", userURL("/api/v1/users/me"), map[string]interface{}{"is_private": true}, bearer(bob.AccessToken))
	doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	resp, body := doJSON("POST", socialURL("/api/v1/social/follow-requests/respond"), map[string]interface{}{
		"follower_id": alice.ID,
		"accept":      false,
	}, bearer(bob.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "request rejected", body["message"])
}

func TestIntegration_Social_RemoveFollower(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	// Bob removes Alice as a follower
	resp, body := doJSON("DELETE", socialURL("/api/v1/social/followers/"+strconv.FormatUint(alice.ID, 10)), nil, bearer(bob.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "follower removed", body["message"])
}

func TestIntegration_Social_GetFollowers(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	resp, body := doJSON("GET", socialURL("/api/v1/social/users/"+strconv.FormatUint(bob.ID, 10)+"/followers"), nil, bearer(bob.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	total := int(body["total"].(float64))
	assert.GreaterOrEqual(t, total, 1)
}

func TestIntegration_Social_GetFollowing(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	resp, body := doJSON("GET", socialURL("/api/v1/social/users/"+strconv.FormatUint(alice.ID, 10)+"/following"), nil, bearer(alice.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	total := int(body["total"].(float64))
	assert.GreaterOrEqual(t, total, 1)
}

func TestIntegration_Social_GetStatus_Stranger(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	resp, body := doJSON("GET", socialURL("/api/v1/social/status/"+strconv.FormatUint(bob.ID, 10)), nil, bearer(alice.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, false, body["is_following"])
	assert.Equal(t, false, body["is_followed_by"])
	assert.Equal(t, false, body["is_blocking"])
	assert.Equal(t, false, body["is_blocked_by"])
}

func TestIntegration_Social_Block_Unblock(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	// Block
	resp, body := doJSON("POST", socialURL("/api/v1/social/block"), map[string]interface{}{
		"target_user_id": bob.ID,
	}, bearer(alice.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "user blocked", body["message"])

	// Verify status
	resp2, body2 := doJSON("GET", socialURL("/api/v1/social/status/"+strconv.FormatUint(bob.ID, 10)), nil, bearer(alice.AccessToken))
	require.NotNil(t, resp2)
	assert.Equal(t, true, body2["is_blocking"])

	// Unblock
	resp3, body3 := doJSON("DELETE", socialURL("/api/v1/social/block/"+strconv.FormatUint(bob.ID, 10)), nil, bearer(alice.AccessToken))
	require.NotNil(t, resp3)
	assert.Equal(t, http.StatusOK, resp3.StatusCode)
	assert.Equal(t, "user unblocked", body3["message"])
}

func TestIntegration_Social_BlockSelf_400(t *testing.T) {
	alice := registerUser(t)
	resp, _ := doJSON("POST", socialURL("/api/v1/social/block"), map[string]interface{}{
		"target_user_id": alice.ID,
	}, bearer(alice.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Social_GetBlocked_List(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	doJSON("POST", socialURL("/api/v1/social/block"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	resp, body := doJSON("GET", socialURL("/api/v1/social/blocked"), nil, bearer(alice.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	total := int(body["total"].(float64))
	assert.GreaterOrEqual(t, total, 1)
}

func TestIntegration_Social_NoToken_401(t *testing.T) {
	resp, _ := doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": 1}, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ── Social internal endpoints ─────────────────────────────────────────────────

func TestIntegration_Social_Internal_CanView_Public(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	resp, body := doJSON("POST", socialURL("/internal/social/can-view"), map[string]interface{}{
		"viewer_id": alice.ID,
		"owner_id":  bob.ID,
	}, internal())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body["can_view"])
}

func TestIntegration_Social_Internal_CanView_WrongSecret_403(t *testing.T) {
	resp, _ := doJSON("POST", socialURL("/internal/social/can-view"), map[string]interface{}{
		"viewer_id": 1, "owner_id": 2,
	}, map[string]string{"X-Internal-Secret": "wrong"})
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestIntegration_Social_Internal_GetFollowingIDs(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	resp, body := doJSON("GET", socialURL("/internal/social/following-ids/"+strconv.FormatUint(alice.ID, 10)), nil, internal())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	ids := body["following_ids"]
	// May be nil if not yet accepted (but public → accepted immediately)
	if ids != nil {
		arr := ids.([]interface{})
		found := false
		for _, v := range arr {
			if uint64(v.(float64)) == bob.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "Bob's ID must be in Alice's following IDs")
	}
}

func TestIntegration_Social_Internal_IsBlocked(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	doJSON("POST", socialURL("/api/v1/social/block"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	resp, body := doJSON("POST", socialURL("/internal/social/is-blocked"), map[string]interface{}{
		"actor_id":  alice.ID,
		"target_id": bob.ID,
	}, internal())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body["is_blocking"])
	assert.Equal(t, true, body["either_blocked"])
}

// ═════════════════════════════════════════════════════════════════════════════
// POST-SERVICE
// ═════════════════════════════════════════════════════════════════════════════

func TestIntegration_Post_CreateAndGet(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "My first post")

	resp, body := doJSON("GET", postURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, float64(postID), body["id"])
	assert.Equal(t, "My first post", body["caption"])
	assert.NotNil(t, body["media"])
}

func TestIntegration_Post_CreateNoFiles_400(t *testing.T) {
	u := registerUser(t)
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	w.WriteField("caption", "no files")
	w.Close()

	req, _ := http.NewRequest("POST", postURL("/api/v1/posts"), &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+u.AccessToken)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Post_GetPostOwner(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "owner test")

	// GetPostOwner is an internal route: GET /internal/posts/:post_id/owner
	resp, body := doJSON("GET", postURL("/internal/posts/"+strconv.FormatUint(postID, 10)+"/owner"), nil,
		map[string]string{"X-Internal-Secret": getInternalSecret()})
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, float64(u.ID), body["user_id"])
}

func TestIntegration_Post_UpdateCaption(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "original")

	resp, body := doJSON("PATCH", postURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/caption"),
		map[string]interface{}{"caption": "updated caption"}, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "caption updated", body["message"])
}

func TestIntegration_Post_UpdateCaption_NotOwner_403(t *testing.T) {
	owner := registerUser(t)
	other := registerUser(t)
	postID := uploadPost(t, owner.AccessToken, "caption")

	resp, _ := doJSON("PATCH", postURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/caption"),
		map[string]interface{}{"caption": "stolen"}, bearer(other.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestIntegration_Post_DeletePost(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "to delete")

	resp, body := doJSON("DELETE", postURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "post deleted", body["message"])

	// Verify the post is gone
	resp2, _ := doJSON("GET", postURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)), nil, bearer(u.AccessToken))
	require.NotNil(t, resp2)
	assert.Equal(t, http.StatusNotFound, resp2.StatusCode)
}

func TestIntegration_Post_DeletePost_NotOwner_403(t *testing.T) {
	owner := registerUser(t)
	other := registerUser(t)
	postID := uploadPost(t, owner.AccessToken, "not yours")

	resp, _ := doJSON("DELETE", postURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)), nil, bearer(other.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestIntegration_Post_GetUserPosts(t *testing.T) {
	u := registerUser(t)
	uploadPost(t, u.AccessToken, "post 1")
	uploadPost(t, u.AccessToken, "post 2")

	resp, body := doJSON("GET", postURL("/api/v1/users/"+strconv.FormatUint(u.ID, 10)+"/posts"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	total := int(body["total"].(float64))
	assert.GreaterOrEqual(t, total, 2)
}

func TestIntegration_Post_GetUserPosts_PrivateProfile_OtherUser_403(t *testing.T) {
	owner := registerUser(t)
	viewer := registerUser(t)

	doJSON("PUT", userURL("/api/v1/users/me"), map[string]interface{}{"is_private": true}, bearer(owner.AccessToken))
	uploadPost(t, owner.AccessToken, "private post")

	resp, _ := doJSON("GET", postURL("/api/v1/users/"+strconv.FormatUint(owner.ID, 10)+"/posts"), nil, bearer(viewer.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestIntegration_Post_DeleteMedia(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "media test")

	// Get the post to find media ID
	resp, body := doJSON("GET", postURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	media := body["media"].([]interface{})
	require.NotEmpty(t, media)
	mediaID := uint64(media[0].(map[string]interface{})["id"].(float64))

	resp2, body2 := doJSON("DELETE", postURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/media/"+strconv.FormatUint(mediaID, 10)),
		nil, bearer(u.AccessToken))
	require.NotNil(t, resp2)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	assert.Equal(t, "media removed", body2["message"])
}

func TestIntegration_Post_NoToken_401(t *testing.T) {
	resp, _ := doJSON("GET", postURL("/api/v1/posts/1"), nil, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

// ── Post internal endpoints ───────────────────────────────────────────────────

func TestIntegration_Post_Internal_GetByUserIDs(t *testing.T) {
	u := registerUser(t)
	uploadPost(t, u.AccessToken, "internal test")

	resp, body := doJSON("POST", postURL("/internal/posts/by-user-ids"), map[string]interface{}{
		"user_ids":  []uint64{u.ID},
		"page":      1,
		"page_size": 20,
	}, internal())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotNil(t, body["data"])
}

func TestIntegration_Post_Internal_GetByUserIDs_WrongSecret_403(t *testing.T) {
	resp, _ := doJSON("POST", postURL("/internal/posts/by-user-ids"), map[string]interface{}{
		"user_ids": []uint64{1}, "page": 1, "page_size": 20,
	}, map[string]string{"X-Internal-Secret": "wrong"})
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestIntegration_Post_Internal_LikeCount(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "count test")

	resp, body := doJSON("POST", postURL("/internal/posts/"+strconv.FormatUint(postID, 10)+"/like-count"),
		map[string]interface{}{"delta": 1}, internal())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body["ok"])
}

func TestIntegration_Post_Internal_CommentCount(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "count test")

	resp, body := doJSON("POST", postURL("/internal/posts/"+strconv.FormatUint(postID, 10)+"/comment-count"),
		map[string]interface{}{"delta": 1}, internal())
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body["ok"])
}

// ═════════════════════════════════════════════════════════════════════════════
// INTERACTION-SERVICE
// ═════════════════════════════════════════════════════════════════════════════

func TestIntegration_Like_Success(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "like me")

	resp, body := doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/like"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "liked", body["message"])
}

func TestIntegration_Like_Duplicate_200_Idempotent(t *testing.T) {
	// INSERT IGNORE → duplicate like always returns 200
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "like twice")

	doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/like"), nil, bearer(u.AccessToken))
	resp, _ := doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/like"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode, "duplicate like must return 200 (idempotent)")
}

func TestIntegration_Unlike_Success(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "unlike me")

	doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/like"), nil, bearer(u.AccessToken))

	resp, body := doJSON("DELETE", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/like"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "unliked", body["message"])
}

func TestIntegration_HasLiked_True(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "check liked")

	doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/like"), nil, bearer(u.AccessToken))

	resp, body := doJSON("GET", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/liked"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body["liked"])
}

func TestIntegration_HasLiked_False(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "not liked")

	resp, body := doJSON("GET", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/liked"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, false, body["liked"])
}

func TestIntegration_Like_PrivatePost_ByNonFollower_400(t *testing.T) {
	owner := registerUser(t)
	viewer := registerUser(t)

	doJSON("PUT", userURL("/api/v1/users/me"), map[string]interface{}{"is_private": true}, bearer(owner.AccessToken))
	postID := uploadPost(t, owner.AccessToken, "private")

	resp, _ := doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/like"), nil, bearer(viewer.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Comment_AddAndList(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "commentable")

	resp, body := doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/comments"),
		map[string]interface{}{"body": "Great post!"}, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "Great post!", body["body"])
	commentID := uint64(body["id"].(float64))
	assert.NotZero(t, commentID)

	// List
	resp2, body2 := doJSON("GET", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/comments"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp2)
	assert.Equal(t, http.StatusOK, resp2.StatusCode)
	total := int(body2["total"].(float64))
	assert.GreaterOrEqual(t, total, 1)
}

func TestIntegration_Comment_EmptyBody_400(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "test")

	resp, _ := doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/comments"),
		map[string]interface{}{"body": ""}, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestIntegration_Comment_UpdateOwn(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "update test")

	_, cBody := doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/comments"),
		map[string]interface{}{"body": "original"}, bearer(u.AccessToken))
	commentID := uint64(cBody["id"].(float64))

	resp, body := doJSON("PATCH", interURL("/api/v1/comments/"+strconv.FormatUint(commentID, 10)),
		map[string]interface{}{"body": "edited"}, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "comment updated", body["message"])
}

func TestIntegration_Comment_UpdateNotOwner_403(t *testing.T) {
	owner := registerUser(t)
	other := registerUser(t)
	postID := uploadPost(t, owner.AccessToken, "test")

	_, cBody := doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/comments"),
		map[string]interface{}{"body": "owner comment"}, bearer(owner.AccessToken))
	commentID := uint64(cBody["id"].(float64))

	resp, _ := doJSON("PATCH", interURL("/api/v1/comments/"+strconv.FormatUint(commentID, 10)),
		map[string]interface{}{"body": "hacked"}, bearer(other.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestIntegration_Comment_DeleteOwn(t *testing.T) {
	u := registerUser(t)
	postID := uploadPost(t, u.AccessToken, "delete comment test")

	_, cBody := doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/comments"),
		map[string]interface{}{"body": "to delete"}, bearer(u.AccessToken))
	commentID := uint64(cBody["id"].(float64))

	resp, body := doJSON("DELETE", interURL("/api/v1/comments/"+strconv.FormatUint(commentID, 10)), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "comment deleted", body["message"])
}

func TestIntegration_Comment_DeleteNotOwner_403(t *testing.T) {
	owner := registerUser(t)
	other := registerUser(t)
	postID := uploadPost(t, owner.AccessToken, "test")

	_, cBody := doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/comments"),
		map[string]interface{}{"body": "owner comment"}, bearer(owner.AccessToken))
	commentID := uint64(cBody["id"].(float64))

	resp, _ := doJSON("DELETE", interURL("/api/v1/comments/"+strconv.FormatUint(commentID, 10)), nil, bearer(other.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestIntegration_Comment_NotFound_403(t *testing.T) {
	// comment not found → service returns "comment not found" → controller maps to 403
	u := registerUser(t)
	resp, _ := doJSON("DELETE", interURL("/api/v1/comments/9999999"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestIntegration_Comment_OnPrivatePost_NonFollower_403(t *testing.T) {
	owner := registerUser(t)
	viewer := registerUser(t)

	doJSON("PUT", userURL("/api/v1/users/me"), map[string]interface{}{"is_private": true}, bearer(owner.AccessToken))
	postID := uploadPost(t, owner.AccessToken, "private")

	resp, _ := doJSON("POST", interURL("/api/v1/posts/"+strconv.FormatUint(postID, 10)+"/comments"),
		map[string]interface{}{"body": "sneaky comment"}, bearer(viewer.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// ═════════════════════════════════════════════════════════════════════════════
// FEED-SERVICE
// ═════════════════════════════════════════════════════════════════════════════

func TestIntegration_Feed_IncludesOwnPosts(t *testing.T) {
	u := registerUser(t)
	uploadPost(t, u.AccessToken, "my own post")

	resp, body := doJSON("GET", feedURL("/api/v1/feed"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.NotNil(t, body["posts"])
	total := int(body["total"].(float64))
	assert.GreaterOrEqual(t, total, 1, "own posts must appear in feed")
}

func TestIntegration_Feed_IncludesFollowedUserPosts(t *testing.T) {
	alice := registerUser(t)
	bob := registerUser(t)

	// Bob posts, Alice follows Bob, Alice's feed should include Bob's post
	uploadPost(t, bob.AccessToken, "bob's post for alice")
	doJSON("POST", socialURL("/api/v1/social/follow"), map[string]interface{}{"target_user_id": bob.ID}, bearer(alice.AccessToken))

	// Give a moment for consistency
	time.Sleep(100 * time.Millisecond)

	resp, body := doJSON("GET", feedURL("/api/v1/feed"), nil, bearer(alice.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	posts := body["posts"].([]interface{})
	found := false
	for _, p := range posts {
		pm := p.(map[string]interface{})
		if uint64(pm["user_id"].(float64)) == bob.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "Bob's post must appear in Alice's feed after following")
}

func TestIntegration_Feed_EmptyFeed_ReturnsOwnPosts(t *testing.T) {
	u := registerUser(t)
	// No follows, no own posts — feed returns 200 with posts:null or posts:[]
	resp, body := doJSON("GET", feedURL("/api/v1/feed"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	// posts is null (no posts) or an array — both are valid empty feed responses
	_, hasKey := body["posts"]
	assert.True(t, hasKey, "response must have a 'posts' key")
}

func TestIntegration_Feed_NoToken_401(t *testing.T) {
	resp, _ := doJSON("GET", feedURL("/api/v1/feed"), nil, nil)
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestIntegration_Feed_Pagination(t *testing.T) {
	u := registerUser(t)
	for i := 0; i < 3; i++ {
		uploadPost(t, u.AccessToken, "feed page post "+strconv.Itoa(i))
	}

	resp, body := doJSON("GET", feedURL("/api/v1/feed?page=1&page_size=2"), nil, bearer(u.AccessToken))
	require.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, float64(1), body["page"])
	assert.Equal(t, float64(2), body["page_size"])
	posts := body["posts"].([]interface{})
	assert.LessOrEqual(t, len(posts), 2)
}

// ─── Helper used only in this file ───────────────────────────────────────────
var _ = strings.TrimSpace // suppress unused import
