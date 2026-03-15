.PHONY: all build up down logs clean tidy

all: build

build:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

clean:
	docker compose down -v --remove-orphans

db:
	docker compose up -d mysql minio

tidy:
	cd auth-service        && go mod tidy
	cd user-service        && go mod tidy
	cd social-service      && go mod tidy
	cd post-service        && go mod tidy
	cd interaction-service && go mod tidy
	cd feed-service        && go mod tidy

## Run individual services locally
auth:
	cd auth-service && go run ./cmd/main.go
user:
	cd user-service && go run ./cmd/main.go
social:
	cd social-service && go run ./cmd/main.go
post:
	cd post-service && go run ./cmd/main.go
interaction:
	cd interaction-service && go run ./cmd/main.go
feed:
	cd feed-service && go run ./cmd/main.go
