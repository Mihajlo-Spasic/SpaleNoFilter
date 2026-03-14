# Instagram Clone — Microservices Architecture

A scalable **Instagram-like social network** built using a **microservices architecture**.  
The platform allows users to register, follow other users, create posts with media, like and comment on posts, and view a personalized timeline feed.

The project demonstrates modern **DevOps practices**, including containerization, CI pipelines, automated testing, and service separation.

---

# Architecture

The application is composed of independent **microservices**, each responsible for a specific domain.

| Service | Port | Responsibility |
|------|------|------|
| user-service | 8080 | Authentication, user profiles, search |
| social-service | 8082 | Follow system, follow requests, blocking |
| post-service | 8083 | Post creation, media uploads |
| interaction-service | 8084 | Likes and comments |
| feed-service | 8085 | Timeline feed generation |

Media files are stored in **MinIO object storage**.

| Tool | Port |
|-----|-----|
| MinIO API | 9000 |
| MinIO Console | 9001 |

---

# Technology Stack

### Backend
- Go (Golang)
- REST API
- JWT Authentication
- Microservices architecture

### Database
- MySQL

### Storage
- MinIO (object storage for images and videos)

### DevOps
- Docker
- Docker Compose
- GitHub Actions (CI)

### Testing
- Unit tests
- Integration tests (API)
- UI integration tests

---

# Main Features

## User Authentication

- User registration
- Login with username or email
- JWT authentication
- Token refresh
- Logout

---

## User Profiles

Each profile contains:

- full name
- username
- profile description
- profile picture
- public or private account option

---

## Follow System

### Public Profiles
Follow requests are **automatically accepted**.

### Private Profiles
Follow requests must be **manually approved**.

Users can:

- follow other users
- unfollow users
- remove followers

---

## Blocking

Users can block other users.

Blocked users:

- cannot find the profile in search
- cannot view posts
- cannot follow the user
- existing follow relationships are automatically removed

---

## Posts

Users can publish posts containing:

- images
- videos

Constraints:

- maximum **20 media files per post**
- maximum **50 MB per file**

Posts support:

- caption editing
- removing media from a carousel
- deleting posts

---

## Likes

Users can:

- like posts
- remove likes

Each post displays the **total number of likes**.

---

## Comments

Users can:

- add comments
- edit their comments
- delete their comments

Each post displays the **total number of comments**.

---

## Timeline Feed

The home page shows a **chronological feed** containing posts from:

- users you follow
- your own posts

Newest posts appear first.

---

## User Search

Users can search profiles by:

- username
- full name

---

# Running the Application

Start all services using Docker Compose.

```bash
docker compose up --build
```

Services will be available on:

```
user-service        http://localhost:8080
social-service      http://localhost:8082
post-service        http://localhost:8083
interaction-service http://localhost:8084
feed-service        http://localhost:8085
```

---

# Swagger API Documentation

Each service has its own Swagger specification located in the `swagger/` directory.

Open them using:

https://editor.swagger.io

| Spec File | Port | Service |
|------|------|------|
| swagger/01-user-service.yaml | 8080 | Auth, profiles, search |
| swagger/02-social-service.yaml | 8082 | Follow, block |
| swagger/03-post-service.yaml | 8083 | Posts, media upload |
| swagger/04-interaction-service.yaml | 8084 | Likes, comments |
| swagger/05-feed-service.yaml | 8085 | Timeline feed |

---

# API Usage Guide

## Step 1 — Register users

Example:

```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
-H "Content-Type: application/json" \
-d '{"username":"alice","email":"alice@example.com","password":"password123","full_name":"Alice"}'
```

---

## Step 2 — Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
-H "Content-Type: application/json" \
-d '{"username_or_email":"alice","password":"password123"}'
```

The response returns:

- access_token
- refresh_token

---

## Step 3 — Follow user

```
POST http://localhost:8082/api/v1/social/follow
```

---

## Step 4 — Create post

Post creation uses **multipart/form-data**.

```bash
curl -X POST http://localhost:8083/api/v1/posts \
-H "Authorization: Bearer <TOKEN>" \
-F "files=@photo.jpg" \
-F "caption=Beautiful sunset"
```

---

## Step 5 — Like post

```
POST /api/v1/posts/{post_id}/like
```

---

## Step 6 — Comment on post

```
POST /api/v1/posts/{post_id}/comments
```

---

## Step 7 — View feed

```
GET /api/v1/feed
```

---

# Running Tests

## Run all tests

```bash
INTERNAL_SECRET=internal-service-to-service-secret-key go test ./... -timeout 120s
```

## Run only unit tests

```bash
go test ./... -run "^Test[^I]"
```

## Run integration tests

```bash
INTERNAL_SECRET=internal-service-to-service-secret-key go test ./... -run "^TestIntegration" -timeout 120s
```

---

# Continuous Integration

The CI pipeline performs:

- running unit tests
- running integration tests
- building Docker images
- publishing Docker images to a registry

---

# DevOps Features

The project demonstrates modern DevOps concepts:

- microservices architecture
- containerized services
- object storage integration
- CI automation
- automated testing
- scalable service separation

---
## Team

Project developed by:

- **Anita Mijatović**
- **Mihajlo Spasić**
- **Mihailo Šundović**

---

# License

This project was created for educational purposes.
