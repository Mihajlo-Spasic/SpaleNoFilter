# Instagram Clone — API Usage Guide

## How to open the Swagger UI

Each service has its own spec file in the `swagger/` folder. Load each one at
[https://editor.swagger.io](https://editor.swagger.io) — the "Try it out" button
will use the correct port automatically.

| Spec file | Port | Service |
|---|---|---|
| `swagger/01-user-service.yaml` | 8080 | Auth, profiles, search |
| `swagger/02-social-service.yaml` | 8082 | Follow, block |
| `swagger/03-post-service.yaml` | 8083 | Posts, media upload |
| `swagger/04-interaction-service.yaml` | 8084 | Likes, comments |
| `swagger/05-feed-service.yaml` | 8085 | Timeline feed |

---

## Step-by-step API call order

### Step 1 — Register two users
You need two users to test social features meaningfully.

```bash
# User A
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","email":"alice@example.com","password":"password123","full_name":"Alice"}' \
  | jq .

# User B
curl -s -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"bob","email":"bob@example.com","password":"password123","full_name":"Bob"}' \
  | jq .
```

**Save from the response:**
- `access_token` → use as `Authorization: Bearer <token>` on every protected endpoint
- `refresh_token` → store securely, used to get a new access token when it expires (15 min)
- `user.id` → you'll need this for social calls

---

### Step 2 — Login (if you already registered)

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username_or_email":"alice","password":"password123"}' \
  | jq .
```

---

### Step 3 — View and update your profile

```bash
# Get own profile (includes email, role)
curl -s http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer <ALICE_TOKEN>" | jq .

# Update bio and set account to private
curl -s -X PUT http://localhost:8080/api/v1/users/me \
  -H "Authorization: Bearer <ALICE_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"bio":"Just here for the vibes","is_private":false}' | jq .

# View someone else's public profile (no token required)
curl -s http://localhost:8080/api/v1/users/bob/profile | jq .
```

---

### Step 4 — Follow another user

Alice follows Bob. Because Bob is public, the follow is accepted immediately.

```bash
# Get Bob's user ID first (from his profile)
BOB_ID=$(curl -s http://localhost:8080/api/v1/users/bob/profile | jq .id)

# Alice follows Bob
curl -s -X POST http://localhost:8082/api/v1/social/follow \
  -H "Authorization: Bearer <ALICE_TOKEN>" \
  -H "Content-Type: application/json" \
  -d "{\"target_user_id\": $BOB_ID}" | jq .

# Check the relationship status
curl -s http://localhost:8082/api/v1/social/status/$BOB_ID \
  -H "Authorization: Bearer <ALICE_TOKEN>" | jq .
# → is_following: true, follow_status: "accepted"
```

**If Bob's profile were private**, the response would be `status: pending`.
Bob would then need to accept it:

```bash
ALICE_ID=$(curl -s http://localhost:8080/api/v1/users/alice/profile | jq .id)

# Bob sees pending requests
curl -s http://localhost:8082/api/v1/social/follow-requests/pending \
  -H "Authorization: Bearer <BOB_TOKEN>" | jq .

# Bob accepts Alice's request
curl -s -X POST http://localhost:8082/api/v1/social/follow-requests/respond \
  -H "Authorization: Bearer <BOB_TOKEN>" \
  -H "Content-Type: application/json" \
  -d "{\"follower_id\": $ALICE_ID, \"accept\": true}" | jq .
```

---

### Step 5 — Create a post with media

Post-service handles multipart file uploads. Files go directly to MinIO.

```bash
# Create a post with one image and a caption
curl -s -X POST http://localhost:8083/api/v1/posts \
  -H "Authorization: Bearer <ALICE_TOKEN>" \
  -F "files=@/path/to/photo.jpg" \
  -F "caption=Golden hour 🌅" | jq .
```

**Save from the response:**
- `id` → the post ID, needed for likes/comments
- `media[0].media_url` → the direct MinIO URL to the uploaded image

For multiple files (carousel post):

```bash
curl -s -X POST http://localhost:8083/api/v1/posts \
  -H "Authorization: Bearer <ALICE_TOKEN>" \
  -F "files=@photo1.jpg" \
  -F "files=@photo2.jpg" \
  -F "files=@video.mp4" \
  -F "caption=Weekend trip 🏔️" | jq .
```

---

### Step 6 — View posts

```bash
# Get a specific post (full detail + all media)
POST_ID=1
curl -s http://localhost:8083/api/v1/posts/$POST_ID \
  -H "Authorization: Bearer <BOB_TOKEN>" | jq .

# Get Alice's post gallery (grid of thumbnails)
ALICE_ID=1
curl -s "http://localhost:8083/api/v1/users/$ALICE_ID/posts?page=1&page_size=12" \
  -H "Authorization: Bearer <BOB_TOKEN>" | jq .
```

---

### Step 7 — Like and comment

```bash
POST_ID=1

# Bob likes Alice's post
curl -s -X POST http://localhost:8084/api/v1/posts/$POST_ID/like \
  -H "Authorization: Bearer <BOB_TOKEN>" | jq .

# Check if Bob liked it
curl -s http://localhost:8084/api/v1/posts/$POST_ID/liked \
  -H "Authorization: Bearer <BOB_TOKEN>" | jq .
# → {"liked": true}

# Bob comments
curl -s -X POST http://localhost:8084/api/v1/posts/$POST_ID/comments \
  -H "Authorization: Bearer <BOB_TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"body":"Amazing shot! 🔥"}' | jq .

# Read all comments
curl -s "http://localhost:8084/api/v1/posts/$POST_ID/comments?page=1" \
  -H "Authorization: Bearer <ALICE_TOKEN>" | jq .
```

---

### Step 8 — View the feed

The feed returns posts from everyone Alice follows, plus her own, newest first.

```bash
curl -s "http://localhost:8085/api/v1/feed?page=1&page_size=20" \
  -H "Authorization: Bearer <ALICE_TOKEN>" | jq .
```

---

### Step 9 — Token refresh

Access tokens expire after **15 minutes**. When you get a `401`, refresh:

```bash
curl -s -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token":"<ALICE_REFRESH_TOKEN>"}' | jq .
```

Save the new `access_token` and `refresh_token` — the old refresh token is now invalid.

---

### Step 10 — Block a user

```bash
BOB_ID=2

# Alice blocks Bob — removes any follow in both directions
curl -s -X POST http://localhost:8082/api/v1/social/block \
  -H "Authorization: Bearer <ALICE_TOKEN>" \
  -H "Content-Type: application/json" \
  -d "{\"target_user_id\": $BOB_ID}" | jq .

# Unblock later
curl -s -X DELETE http://localhost:8082/api/v1/social/block/$BOB_ID \
  -H "Authorization: Bearer <ALICE_TOKEN>" | jq .
```

---

## Quick reference — all ports

| Port | Service | What you call here |
|---|---|---|
| **8080** | user-service | Register, login, refresh, logout, profiles, search |
| **8082** | social-service | Follow, unfollow, follow requests, block |
| **8083** | post-service | Create post (multipart), get post, get gallery |
| **8084** | interaction-service | Like/unlike, add/read/edit/delete comments |
| **8085** | feed-service | Home timeline |
| **9000** | MinIO | Media files served directly (URLs in post responses) |
| **9001** | MinIO console | `http://localhost:9001` — browse uploaded files visually |

## Common mistakes

- **Wrong port** — social calls go to `:8082`, not `:8080`. Each service is independent.
- **No Authorization header** — every endpoint except register, login, refresh, and public profile requires `Authorization: Bearer <token>`.
- **Multipart for posts** — post creation uses `multipart/form-data`, not JSON. Use `-F` in curl, not `-d`.
- **init.sql re-run** — if you change `.env` DB credentials, you must `docker compose down -v` (deletes the volume) then `make up` so init.sql runs again with the new user.
- **Token expired** — access tokens last 15 min. Refresh tokens last 7 days. Always handle 401s by refreshing.
