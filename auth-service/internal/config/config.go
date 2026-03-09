package config

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	Port           string
	DBHost         string
	DBPort         string
	DBUser         string
	DBPassword     string
	DBName         string
	JWTSecret      string
	JWTExpiry      time.Duration
	RefreshExpiry  time.Duration
	InternalSecret string
}

func Load() *Config {
	jwtExpiry, _ := time.ParseDuration(getEnv("JWT_EXPIRY", "15m"))
	refreshExpiry, _ := time.ParseDuration(getEnv("REFRESH_EXPIRY", "168h")) // 7 days

	return &Config{
		Port:           getEnv("PORT", "8081"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "3306"),
		DBUser:         getEnv("DB_USER", "spale"),
		DBPassword:     getEnv("DB_PASSWORD", "spale"),
		DBName:         getEnv("DB_NAME", "instagram_auth"),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production"),
		JWTExpiry:      jwtExpiry,
		RefreshExpiry:  refreshExpiry,
		InternalSecret: getEnv("INTERNAL_SECRET", "internal-secret-key"),
	}
}

func NewMySQLConnection(cfg *Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	log.Println("Connected to MySQL successfully")
	return db, nil
}

func RunMigrations(db *sql.DB) error {
	migrations := []string{
		`CREATE TABLE IF NOT EXISTS refresh_tokens (
			id           BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			user_id      BIGINT UNSIGNED NOT NULL,
			token_hash   VARCHAR(255) NOT NULL UNIQUE,
			user_agent   VARCHAR(500),
			ip_address   VARCHAR(45),
			expires_at   DATETIME NOT NULL,
			revoked      BOOLEAN NOT NULL DEFAULT FALSE,
			revoked_at   DATETIME,
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_user_id (user_id),
			INDEX idx_token_hash (token_hash),
			INDEX idx_expires_at (expires_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS token_blacklist (
			id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			jti        VARCHAR(255) NOT NULL UNIQUE,
			expires_at DATETIME NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_jti (jti),
			INDEX idx_expires_at (expires_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	log.Println("Auth migrations ran successfully")
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
