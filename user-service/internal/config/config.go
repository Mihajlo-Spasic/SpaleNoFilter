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
	AuthServiceURL string
	InternalSecret string
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8080"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "3306"),
		DBUser:         getEnv("DB_USER", "spale"),
		DBPassword:     getEnv("DB_PASSWORD", "Spale"),
		DBName:         getEnv("DB_NAME", "instagram_users"),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production"),
		AuthServiceURL: getEnv("AUTH_SERVICE_URL", "http://localhost:8081"),
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
		`CREATE TABLE IF NOT EXISTS users (
			id              BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			username        VARCHAR(30)  NOT NULL UNIQUE,
			email           VARCHAR(255) NOT NULL UNIQUE,
			password_hash   VARCHAR(255) NOT NULL,
			full_name       VARCHAR(100) NOT NULL DEFAULT '',
			bio             VARCHAR(1000) NOT NULL DEFAULT '',
			avatar_url      VARCHAR(500) NOT NULL DEFAULT '',
			website         VARCHAR(255) NOT NULL DEFAULT '',
			is_private      BOOLEAN NOT NULL DEFAULT FALSE,
			is_verified     BOOLEAN NOT NULL DEFAULT FALSE,
			is_active       BOOLEAN NOT NULL DEFAULT TRUE,
			role            ENUM('user','moderator','admin') NOT NULL DEFAULT 'user',
			post_count      INT UNSIGNED NOT NULL DEFAULT 0,
			follower_count  INT UNSIGNED NOT NULL DEFAULT 0,
			following_count INT UNSIGNED NOT NULL DEFAULT 0,
			created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			deleted_at      DATETIME,
			INDEX idx_username (username),
			INDEX idx_email (email),
			INDEX idx_is_active (is_active)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`ALTER TABLE users
			MODIFY COLUMN full_name  VARCHAR(100) NOT NULL DEFAULT '',
			MODIFY COLUMN bio        VARCHAR(1000) NOT NULL DEFAULT '',
			MODIFY COLUMN avatar_url VARCHAR(500) NOT NULL DEFAULT '',
			MODIFY COLUMN website    VARCHAR(255) NOT NULL DEFAULT ''`,

		`UPDATE users SET
			full_name  = COALESCE(full_name,  ''),
			bio        = COALESCE(bio,        ''),
			avatar_url = COALESCE(avatar_url, ''),
			website    = COALESCE(website,    '')
		WHERE full_name IS NULL OR bio IS NULL OR avatar_url IS NULL OR website IS NULL`,

		// NOTE: follows and blocks tables live in instagram_social (social-service).
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}

	log.Println("User migrations ran successfully")
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
