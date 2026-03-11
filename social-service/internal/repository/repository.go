package repository

import (
	"database/sql"
	"fmt"

	"github.com/instagram-clone/social-service/internal/models"
)

type FollowRepository interface {
	Upsert(followerID, followingID uint64, status models.FollowStatus) error
	Delete(followerID, followingID uint64) error
	Find(followerID, followingID uint64) (*models.Follow, error)
	UpdateStatus(followerID, followingID uint64, status models.FollowStatus) error

	RemoveFollower(ownerID, followerID uint64) error

	ListFollowers(userID uint64, page, size int) ([]models.FollowEntry, int64, error)
	ListFollowing(userID uint64, page, size int) ([]models.FollowEntry, int64, error)
	ListPendingRequests(userID uint64, page, size int) ([]models.FollowEntry, int64, error)

	GetFollowingIDs(userID uint64) ([]uint64, error)
	IsFollowing(followerID, followingID uint64) (bool, error)
	AcceptedFollowerCount(userID uint64) (int64, error)
	AcceptedFollowingCount(userID uint64) (int64, error)
}

type followRepo struct{ db *sql.DB }

func NewFollowRepository(db *sql.DB) FollowRepository { return &followRepo{db: db} }

func (r *followRepo) Upsert(followerID, followingID uint64, status models.FollowStatus) error {
	_, err := r.db.Exec(
		`INSERT INTO follows (follower_id, following_id, status)
		 VALUES (?, ?, ?)
		 ON DUPLICATE KEY UPDATE status = VALUES(status), updated_at = NOW()`,
		followerID, followingID, status,
	)
	return err
}

func (r *followRepo) Delete(followerID, followingID uint64) error {
	_, err := r.db.Exec(
		`DELETE FROM follows WHERE follower_id = ? AND following_id = ?`,
		followerID, followingID,
	)
	return err
}

func (r *followRepo) Find(followerID, followingID uint64) (*models.Follow, error) {
	f := &models.Follow{}
	err := r.db.QueryRow(
		`SELECT id, follower_id, following_id, status, created_at, updated_at
		 FROM follows WHERE follower_id = ? AND following_id = ?`,
		followerID, followingID,
	).Scan(&f.ID, &f.FollowerID, &f.FollowingID, &f.Status, &f.CreatedAt, &f.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return f, err
}

func (r *followRepo) UpdateStatus(followerID, followingID uint64, status models.FollowStatus) error {
	_, err := r.db.Exec(
		`UPDATE follows SET status = ?, updated_at = NOW()
		 WHERE follower_id = ? AND following_id = ?`,
		status, followerID, followingID,
	)
	return err
}

func (r *followRepo) RemoveFollower(ownerID, followerID uint64) error {
	_, err := r.db.Exec(
		`DELETE FROM follows WHERE follower_id = ? AND following_id = ?`,
		followerID, ownerID,
	)
	return err
}

func (r *followRepo) ListFollowers(userID uint64, page, size int) ([]models.FollowEntry, int64, error) {
	var total int64
	r.db.QueryRow(`SELECT COUNT(*) FROM follows WHERE following_id = ? AND status = 'accepted'`, userID).Scan(&total)

	rows, err := r.db.Query(
		`SELECT follower_id, status, created_at FROM follows
		 WHERE following_id = ? AND status = 'accepted'
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		userID, size, (page-1)*size,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanEntries(rows), total, nil
}

func (r *followRepo) ListFollowing(userID uint64, page, size int) ([]models.FollowEntry, int64, error) {
	var total int64
	r.db.QueryRow(`SELECT COUNT(*) FROM follows WHERE follower_id = ? AND status = 'accepted'`, userID).Scan(&total)

	rows, err := r.db.Query(
		`SELECT following_id, status, created_at FROM follows
		 WHERE follower_id = ? AND status = 'accepted'
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		userID, size, (page-1)*size,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanEntries(rows), total, nil
}

func (r *followRepo) ListPendingRequests(userID uint64, page, size int) ([]models.FollowEntry, int64, error) {
	var total int64
	r.db.QueryRow(`SELECT COUNT(*) FROM follows WHERE following_id = ? AND status = 'pending'`, userID).Scan(&total)

	rows, err := r.db.Query(
		`SELECT follower_id, status, created_at FROM follows
		 WHERE following_id = ? AND status = 'pending'
		 ORDER BY created_at ASC LIMIT ? OFFSET ?`,
		userID, size, (page-1)*size,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	return scanEntries(rows), total, nil
}

func (r *followRepo) GetFollowingIDs(userID uint64) ([]uint64, error) {
	rows, err := r.db.Query(
		`SELECT following_id FROM follows WHERE follower_id = ? AND status = 'accepted'`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []uint64
	for rows.Next() {
		var id uint64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *followRepo) IsFollowing(followerID, followingID uint64) (bool, error) {
	var count int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM follows WHERE follower_id = ? AND following_id = ? AND status = 'accepted'`,
		followerID, followingID,
	).Scan(&count)
	return count > 0, err
}

func (r *followRepo) AcceptedFollowerCount(userID uint64) (int64, error) {
	var c int64
	err := r.db.QueryRow(`SELECT COUNT(*) FROM follows WHERE following_id = ? AND status = 'accepted'`, userID).Scan(&c)
	return c, err
}

func (r *followRepo) AcceptedFollowingCount(userID uint64) (int64, error) {
	var c int64
	err := r.db.QueryRow(`SELECT COUNT(*) FROM follows WHERE follower_id = ? AND status = 'accepted'`, userID).Scan(&c)
	return c, err
}

func scanEntries(rows *sql.Rows) []models.FollowEntry {
	var out []models.FollowEntry
	for rows.Next() {
		var e models.FollowEntry
		rows.Scan(&e.UserID, &e.Status, &e.CreatedAt)
		out = append(out, e)
	}
	return out
}

// ─── Block Repository ─────────────────────────────────────────────────────────

type BlockRepository interface {
	Block(blockerID, blockedID uint64) error
	Unblock(blockerID, blockedID uint64) error
	IsBlocked(blockerID, blockedID uint64) (bool, error)
	// Either direction blocked
	EitherBlocked(a, b uint64) (bool, error)
	ListBlocked(blockerID uint64, page, size int) ([]models.FollowEntry, int64, error)
}

type blockRepo struct{ db *sql.DB }

func NewBlockRepository(db *sql.DB) BlockRepository { return &blockRepo{db: db} }

func (r *blockRepo) Block(blockerID, blockedID uint64) error {
	_, err := r.db.Exec(
		`INSERT IGNORE INTO blocks (blocker_id, blocked_id) VALUES (?, ?)`,
		blockerID, blockedID,
	)
	return err
}

func (r *blockRepo) Unblock(blockerID, blockedID uint64) error {
	_, err := r.db.Exec(
		`DELETE FROM blocks WHERE blocker_id = ? AND blocked_id = ?`,
		blockerID, blockedID,
	)
	return err
}

func (r *blockRepo) IsBlocked(blockerID, blockedID uint64) (bool, error) {
	var c int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM blocks WHERE blocker_id = ? AND blocked_id = ?`,
		blockerID, blockedID,
	).Scan(&c)
	return c > 0, err
}

func (r *blockRepo) EitherBlocked(a, b uint64) (bool, error) {
	var c int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM blocks
		 WHERE (blocker_id = ? AND blocked_id = ?) OR (blocker_id = ? AND blocked_id = ?)`,
		a, b, b, a,
	).Scan(&c)
	return c > 0, err
}

func (r *blockRepo) ListBlocked(blockerID uint64, page, size int) ([]models.FollowEntry, int64, error) {
	var total int64
	r.db.QueryRow(`SELECT COUNT(*) FROM blocks WHERE blocker_id = ?`, blockerID).Scan(&total)
	rows, err := r.db.Query(
		`SELECT blocked_id, '', created_at FROM blocks WHERE blocker_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		blockerID, size, (page-1)*size,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list blocked: %w", err)
	}
	defer rows.Close()
	return scanEntries(rows), total, nil
}
