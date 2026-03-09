package repository

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/instagram-clone/auth-service/internal/models"
)

type TokenRepository interface {
	SaveRefreshToken(token *models.RefreshToken) error
	FindRefreshToken(tokenHash string) (*models.RefreshToken, error)
	RevokeRefreshToken(tokenHash string) error
	RevokeAllUserTokens(userID uint64) error
	BlacklistJTI(jti string, expiresAt time.Time) error
	IsJTIBlacklisted(jti string) (bool, error)
	CleanupExpiredTokens() error
}

type tokenRepository struct {
	db *sql.DB
}

func NewTokenRepository(db *sql.DB) TokenRepository {
	return &tokenRepository{db: db}
}

func (r *tokenRepository) SaveRefreshToken(token *models.RefreshToken) error {
	query := `INSERT INTO refresh_tokens 
		(user_id, token_hash, user_agent, ip_address, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, NOW())`

	_, err := r.db.Exec(query,
		token.UserID, token.TokenHash, token.UserAgent,
		token.IPAddress, token.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("save refresh token: %w", err)
	}
	return nil
}

func (r *tokenRepository) FindRefreshToken(tokenHash string) (*models.RefreshToken, error) {
	query := `SELECT id, user_id, token_hash, user_agent, ip_address, 
		expires_at, revoked, revoked_at, created_at
		FROM refresh_tokens WHERE token_hash = ? AND revoked = FALSE`

	row := r.db.QueryRow(query, tokenHash)
	t := &models.RefreshToken{}

	err := row.Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.UserAgent, &t.IPAddress,
		&t.ExpiresAt, &t.Revoked, &t.RevokedAt, &t.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find refresh token: %w", err)
	}
	return t, nil
}

func (r *tokenRepository) RevokeRefreshToken(tokenHash string) error {
	query := `UPDATE refresh_tokens SET revoked = TRUE, revoked_at = NOW() 
		WHERE token_hash = ?`
	_, err := r.db.Exec(query, tokenHash)
	return err
}

func (r *tokenRepository) RevokeAllUserTokens(userID uint64) error {
	query := `UPDATE refresh_tokens SET revoked = TRUE, revoked_at = NOW() 
		WHERE user_id = ? AND revoked = FALSE`
	_, err := r.db.Exec(query, userID)
	return err
}

func (r *tokenRepository) BlacklistJTI(jti string, expiresAt time.Time) error {
	query := `INSERT IGNORE INTO token_blacklist (jti, expires_at, created_at) 
		VALUES (?, ?, NOW())`
	_, err := r.db.Exec(query, jti, expiresAt)
	return err
}

func (r *tokenRepository) IsJTIBlacklisted(jti string) (bool, error) {
	query := `SELECT COUNT(*) FROM token_blacklist WHERE jti = ? AND expires_at > NOW()`
	var count int
	err := r.db.QueryRow(query, jti).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *tokenRepository) CleanupExpiredTokens() error {
	queries := []string{
		`DELETE FROM refresh_tokens WHERE expires_at < NOW()`,
		`DELETE FROM token_blacklist WHERE expires_at < NOW()`,
	}
	for _, q := range queries {
		if _, err := r.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}
