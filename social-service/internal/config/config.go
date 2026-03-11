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
	UserServiceURL string
	InternalSecret string
}

func Load() *Config {
	return &Config{
		Port:           getEnv("PORT", "8082"),
		DBHost:         getEnv("DB_HOST", "localhost"),
		DBPort:         getEnv("DB_PORT", "3306"),
		DBUser:         getEnv("DB_USER", "spale"),
		DBPassword:     getEnv("DB_PASSWORD", "Spale"),
		DBName:         getEnv("DB_NAME", "instagram_social"),
		JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production"),
		UserServiceURL: getEnv("USER_SERVICE_URL", "http://localhost:8080"),
		InternalSecret: getEnv("INTERNAL_SECRET", "internal-secret-key"),
	}
}

func NewMySQLConnection(cfg *Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&charset=utf8mb4&collation=utf8mb4_unicode_ci",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName,
	)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	log.Println("[social] Connected to MySQL")
	return db, nil
}

func RunMigrations(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS follows (
			id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			follower_id   BIGINT UNSIGNED NOT NULL,
			following_id  BIGINT UNSIGNED NOT NULL,
			status        ENUM('pending','accepted') NOT NULL DEFAULT 'pending',
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
			UNIQUE KEY uq_follow (follower_id, following_id),
			INDEX idx_follower  (follower_id),
			INDEX idx_following (following_id),
			INDEX idx_status    (status)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,

		`CREATE TABLE IF NOT EXISTS blocks (
			id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			blocker_id  BIGINT UNSIGNED NOT NULL,
			blocked_id  BIGINT UNSIGNED NOT NULL,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE KEY uq_block (blocker_id, blocked_id),
			INDEX idx_blocker (blocker_id),
			INDEX idx_blocked (blocked_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("migration: %w", err)
		}
	}
	log.Println("[social] Migrations OK")
	return nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
