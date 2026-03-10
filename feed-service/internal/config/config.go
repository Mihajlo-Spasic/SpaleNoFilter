package config

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Config struct {
	Port             string
	JWTSecret        string
	InternalSecret   string
	SocialServiceURL string
	PostServiceURL   string
}

func Load() *Config {
	return &Config{
		Port:             getEnv("PORT", "8085"),
		JWTSecret:        getEnv("JWT_SECRET", "change-me-in-production"),
		InternalSecret:   getEnv("INTERNAL_SECRET", "internal-secret-key"),
		SocialServiceURL: getEnv("SOCIAL_SERVICE_URL", "http://localhost:8082"),
		PostServiceURL:   getEnv("POST_SERVICE_URL", "http://localhost:8083"),
	}
}

func NewHTTPClient() *httpClient {
	return &httpClient{timeout: 5 * time.Second}
}

type httpClient struct{ timeout time.Duration }

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Ensure these are used
var _ = fmt.Sprintf
var _ = log.Println
