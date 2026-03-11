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
	MinioEndpoint                              string
	MinioPublicURL                             string
	MinioAccessKey                             string
	MinioSecretKey                             string
	MinioBucket                                string
	MinioUseSSL                                bool
	MaxFileSizeMB                              int64
	MaxMediaPerPost                            int
}

func Load() *Config {
	return &Config{
		Port:             getEnv("PORT", "8083"),
		DBHost:           getEnv("DB_HOST", "localhost"),
		DBPort:           getEnv("DB_PORT", "3306"),
		DBUser:           getEnv("DB_USER", "spale"),
		DBPassword:       getEnv("DB_PASSWORD", "Spale"),
		DBName:           getEnv("DB_NAME", "instagram_posts"),
		JWTSecret:        getEnv("JWT_SECRET", "change-me-in-production"),
		InternalSecret:   getEnv("INTERNAL_SECRET", "internal-secret-key"),
		SocialServiceURL: getEnv("SOCIAL_SERVICE_URL", "http://localhost:8082"),
		MinioEndpoint:    getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioPublicURL:   getEnv("MINIO_PUBLIC_URL", "http://localhost:9000"),
		MinioAccessKey:   getEnv("MINIO_ACCESS_KEY", "minioadmin"),
		MinioSecretKey:   getEnv("MINIO_SECRET_KEY", "minioadmin"),
		MinioBucket:      getEnv("MINIO_BUCKET", "instagram-media"),
		MaxFileSizeMB:    50,
		MaxMediaPerPost:  20,
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
	log.Println("[post] Connected to MySQL")
	return db, nil
}

func RunMigrations(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS posts (
			id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			user_id     BIGINT UNSIGNED NOT NULL,
			caption     TEXT,
			like_count  INT UNSIGNED NOT NULL DEFAULT 0,
			comment_count INT UNSIGNED NOT NULL DEFAULT 0,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			deleted_at  DATETIME,
			INDEX idx_user_id   (user_id),
			INDEX idx_created   (created_at),
			INDEX idx_deleted   (deleted_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS post_media (
			id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			post_id     BIGINT UNSIGNED NOT NULL,
			media_key   VARCHAR(512) NOT NULL,
			media_url   VARCHAR(1024) NOT NULL,
			media_type  ENUM('image','video') NOT NULL,
			position    TINYINT UNSIGNED NOT NULL DEFAULT 0,
			size_bytes  BIGINT UNSIGNED NOT NULL DEFAULT 0,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_post_id (post_id),
			FOREIGN KEY (post_id) REFERENCES posts(id) ON DELETE CASCADE
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migration: %w", err)
		}
	}
	log.Println("[post] Migrations OK")
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
