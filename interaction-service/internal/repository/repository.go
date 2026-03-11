package repository

import (
	"database/sql"

	"github.com/instagram-clone/interaction-service/internal/models"
)

type LikeRepository interface {
	Like(userID, postID uint64) (bool, error)   // returns true if new like
	Unlike(userID, postID uint64) (bool, error) // returns true if removed
	HasLiked(userID, postID uint64) (bool, error)
	CountByPost(postID uint64) (int64, error)
}

type likeRepo struct{ db *sql.DB }

func NewLikeRepository(db *sql.DB) LikeRepository { return &likeRepo{db} }

func (r *likeRepo) Like(userID, postID uint64) (bool, error) {
	res, err := r.db.Exec(
		`INSERT IGNORE INTO likes (user_id, post_id) VALUES (?, ?)`, userID, postID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *likeRepo) Unlike(userID, postID uint64) (bool, error) {
	res, err := r.db.Exec(
		`DELETE FROM likes WHERE user_id = ? AND post_id = ?`, userID, postID,
	)
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n > 0, nil
}

func (r *likeRepo) HasLiked(userID, postID uint64) (bool, error) {
	var c int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM likes WHERE user_id = ? AND post_id = ?`, userID, postID).Scan(&c)
	return c > 0, err
}

func (r *likeRepo) CountByPost(postID uint64) (int64, error) {
	var c int64
	err := r.db.QueryRow(`SELECT COUNT(*) FROM likes WHERE post_id = ?`, postID).Scan(&c)
	return c, err
}

type CommentRepository interface {
	Create(comment *models.Comment) (*models.Comment, error)
	FindByID(id uint64) (*models.Comment, error)
	Update(id, userID uint64, body string) error
	SoftDelete(id, userID uint64) error
	ListByPost(postID uint64, page, size int) ([]*models.Comment, int64, error)
	// Remove all likes/comments for a blocked user from a post (called internally)
	DeleteByUser(userID uint64) error
}

type commentRepo struct{ db *sql.DB }

func NewCommentRepository(db *sql.DB) CommentRepository { return &commentRepo{db} }

func (r *commentRepo) Create(c *models.Comment) (*models.Comment, error) {
	res, err := r.db.Exec(
		`INSERT INTO comments (post_id, user_id, body) VALUES (?, ?, ?)`,
		c.PostID, c.UserID, c.Body,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.FindByID(uint64(id))
}

func (r *commentRepo) FindByID(id uint64) (*models.Comment, error) {
	c := &models.Comment{}
	err := r.db.QueryRow(
		`SELECT id, post_id, user_id, body, created_at, updated_at
		 FROM comments WHERE id = ? AND deleted_at IS NULL`, id,
	).Scan(&c.ID, &c.PostID, &c.UserID, &c.Body, &c.CreatedAt, &c.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

func (r *commentRepo) Update(id, userID uint64, body string) error {
	_, err := r.db.Exec(
		`UPDATE comments SET body = ? WHERE id = ? AND user_id = ? AND deleted_at IS NULL`,
		body, id, userID,
	)
	return err
}

func (r *commentRepo) SoftDelete(id, userID uint64) error {
	_, err := r.db.Exec(
		`UPDATE comments SET deleted_at = NOW() WHERE id = ? AND user_id = ?`, id, userID,
	)
	return err
}

func (r *commentRepo) ListByPost(postID uint64, page, size int) ([]*models.Comment, int64, error) {
	var total int64
	r.db.QueryRow(`SELECT COUNT(*) FROM comments WHERE post_id = ? AND deleted_at IS NULL`, postID).Scan(&total)

	rows, err := r.db.Query(
		`SELECT id, post_id, user_id, body, created_at, updated_at
		 FROM comments WHERE post_id = ? AND deleted_at IS NULL
		 ORDER BY created_at ASC LIMIT ? OFFSET ?`,
		postID, size, (page-1)*size,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*models.Comment
	for rows.Next() {
		c := &models.Comment{}
		rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.Body, &c.CreatedAt, &c.UpdatedAt)
		out = append(out, c)
	}
	return out, total, nil
}

func (r *commentRepo) DeleteByUser(userID uint64) error {
	_, err := r.db.Exec(`UPDATE comments SET deleted_at = NOW() WHERE user_id = ? AND deleted_at IS NULL`, userID)
	return err
}
