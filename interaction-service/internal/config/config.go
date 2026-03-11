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
	Port                                       string
	DBHost, DBPort, DBUser, DBPassword, DBName string
	JWTSecret                                  string
	InternalSecret                             string
	SocialServiceURL                           string
	PostServiceURL                             string
}

func Load() *Config {
	return &Config{
		Port:             getEnv("PORT", "8084"),
		DBHost:           getEnv("DB_HOST", "localhost"),
		DBPort:           getEnv("DB_PORT", "3306"),
		DBUser:           getEnv("DB_USER", "spale"),
		DBPassword:       getEnv("DB_PASSWORD", "Spale"),
		DBName:           getEnv("DB_NAME", "instagram_interactions"),
		JWTSecret:        getEnv("JWT_SECRET", "change-me-in-production"),
		InternalSecret:   getEnv("INTERNAL_SECRET", "internal-secret-key"),
		SocialServiceURL: getEnv("SOCIAL_SERVICE_URL", "http://localhost:8082"),
		PostServiceURL:   getEnv("POST_SERVICE_URL", "http://localhost:8083"),
	}
}

func NewMySQLConnection(cfg *Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	log.Println("[interaction] Connected to MySQL")
	return db, nil
}

func RunMigrations(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS likes (
			id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			user_id    BIGINT UNSIGNED NOT NULL,
			post_id    BIGINT UNSIGNED NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uq_like (user_id, post_id),
			INDEX idx_post_id (post_id),
			INDEX idx_user_id (user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS comments (
			id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			post_id    BIGINT UNSIGNED NOT NULL,
			user_id    BIGINT UNSIGNED NOT NULL,
			body       TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			deleted_at DATETIME,
			INDEX idx_post_id (post_id),
			INDEX idx_user_id (user_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migration: %w", err)
		}
	}
	log.Println("[interaction] Migrations OK")
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
