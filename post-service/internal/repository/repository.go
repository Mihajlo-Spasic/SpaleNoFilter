package repository

import (
	"database/sql"
	"fmt"

	"github.com/instagram-clone/post-service/internal/models"
)

type PostRepository interface {
	Create(post *models.Post) (*models.Post, error)
	FindByID(id uint64) (*models.Post, error)
	FindMediaByPost(postID uint64) ([]*models.PostMedia, error)
	FindMediaByID(mediaID uint64) (*models.PostMedia, error)
	AddMedia(media *models.PostMedia) error
	DeleteMedia(mediaID, postID uint64) error
	UpdateCaption(postID uint64, caption string) error
	SoftDelete(postID uint64) error
	ListByUser(userID uint64, page, size int) ([]*models.PostSummary, int64, error)
	ListByUserIDs(userIDs []uint64, page, size int) ([]*models.Post, int64, error)
	CountMedia(postID uint64) (int, error)
	IncrementLike(postID uint64, delta int) error
	IncrementComment(postID uint64, delta int) error
}

type postRepo struct{ db *sql.DB }

func NewPostRepository(db *sql.DB) PostRepository { return &postRepo{db: db} }

func (r *postRepo) Create(post *models.Post) (*models.Post, error) {
	res, err := r.db.Exec(
		`INSERT INTO posts (user_id, caption) VALUES (?, ?)`,
		post.UserID, post.Caption,
	)
	if err != nil {
		return nil, fmt.Errorf("create post: %w", err)
	}
	id, _ := res.LastInsertId()
	return r.FindByID(uint64(id))
}

func (r *postRepo) FindByID(id uint64) (*models.Post, error) {
	p := &models.Post{}
	err := r.db.QueryRow(
		`SELECT id, user_id, caption, like_count, comment_count, created_at, updated_at
		 FROM posts WHERE id = ? AND deleted_at IS NULL`, id,
	).Scan(&p.ID, &p.UserID, &p.Caption, &p.LikeCount, &p.CommentCount, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	media, err := r.FindMediaByPost(id)
	if err != nil {
		return nil, err
	}
	p.Media = media
	return p, nil
}

func (r *postRepo) FindMediaByPost(postID uint64) ([]*models.PostMedia, error) {
	rows, err := r.db.Query(
		`SELECT id, post_id, media_key, media_url, media_type, position, size_bytes, created_at
		 FROM post_media WHERE post_id = ? ORDER BY position ASC`, postID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.PostMedia
	for rows.Next() {
		m := &models.PostMedia{}
		rows.Scan(&m.ID, &m.PostID, &m.MediaKey, &m.MediaURL, &m.MediaType, &m.Position, &m.SizeBytes, &m.CreatedAt)
		out = append(out, m)
	}
	return out, nil
}

func (r *postRepo) FindMediaByID(mediaID uint64) (*models.PostMedia, error) {
	m := &models.PostMedia{}
	err := r.db.QueryRow(
		`SELECT id, post_id, media_key, media_url, media_type, position, size_bytes, created_at
		 FROM post_media WHERE id = ?`, mediaID,
	).Scan(&m.ID, &m.PostID, &m.MediaKey, &m.MediaURL, &m.MediaType, &m.Position, &m.SizeBytes, &m.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (r *postRepo) AddMedia(m *models.PostMedia) error {
	_, err := r.db.Exec(
		`INSERT INTO post_media (post_id, media_key, media_url, media_type, position, size_bytes)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		m.PostID, m.MediaKey, m.MediaURL, m.MediaType, m.Position, m.SizeBytes,
	)
	return err
}

func (r *postRepo) DeleteMedia(mediaID, postID uint64) error {
	_, err := r.db.Exec(`DELETE FROM post_media WHERE id = ? AND post_id = ?`, mediaID, postID)
	return err
}

func (r *postRepo) UpdateCaption(postID uint64, caption string) error {
	_, err := r.db.Exec(`UPDATE posts SET caption = ? WHERE id = ? AND deleted_at IS NULL`, caption, postID)
	return err
}

func (r *postRepo) SoftDelete(postID uint64) error {
	_, err := r.db.Exec(`UPDATE posts SET deleted_at = NOW() WHERE id = ?`, postID)
	return err
}

func (r *postRepo) ListByUser(userID uint64, page, size int) ([]*models.PostSummary, int64, error) {
	var total int64
	r.db.QueryRow(`SELECT COUNT(*) FROM posts WHERE user_id = ? AND deleted_at IS NULL`, userID).Scan(&total)

	rows, err := r.db.Query(
		`SELECT p.id, p.user_id, p.like_count, p.comment_count, p.created_at,
			(SELECT media_url FROM post_media WHERE post_id = p.id ORDER BY position LIMIT 1) AS thumb,
			(SELECT COUNT(*) FROM post_media WHERE post_id = p.id) AS media_count
		 FROM posts p WHERE p.user_id = ? AND p.deleted_at IS NULL
		 ORDER BY p.created_at DESC LIMIT ? OFFSET ?`,
		userID, size, (page-1)*size,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []*models.PostSummary
	for rows.Next() {
		s := &models.PostSummary{}
		rows.Scan(&s.ID, &s.UserID, &s.LikeCount, &s.CommentCount, &s.CreatedAt, &s.ThumbnailURL, &s.MediaCount)
		out = append(out, s)
	}
	return out, total, nil
}

func (r *postRepo) ListByUserIDs(userIDs []uint64, page, size int) ([]*models.Post, int64, error) {
	if len(userIDs) == 0 {
		return nil, 0, nil
	}

	// Build IN clause
	in := make([]interface{}, len(userIDs))
	placeholders := ""
	for i, id := range userIDs {
		in[i] = id
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
	}

	var total int64
	countArgs := append([]interface{}{}, in...)
	r.db.QueryRow(
		fmt.Sprintf(`SELECT COUNT(*) FROM posts WHERE user_id IN (%s) AND deleted_at IS NULL`, placeholders),
		countArgs...,
	).Scan(&total)

	args := append(in, size, (page-1)*size)
	rows, err := r.db.Query(
		fmt.Sprintf(`SELECT id, user_id, caption, like_count, comment_count, created_at, updated_at
		 FROM posts WHERE user_id IN (%s) AND deleted_at IS NULL
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`, placeholders),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var posts []*models.Post
	for rows.Next() {
		p := &models.Post{}
		rows.Scan(&p.ID, &p.UserID, &p.Caption, &p.LikeCount, &p.CommentCount, &p.CreatedAt, &p.UpdatedAt)
		posts = append(posts, p)
	}

	// Load media for all posts
	for _, p := range posts {
		p.Media, _ = r.FindMediaByPost(p.ID)
	}
	return posts, total, nil
}

func (r *postRepo) CountMedia(postID uint64) (int, error) {
	var c int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM post_media WHERE post_id = ?`, postID).Scan(&c)
	return c, err
}

func (r *postRepo) IncrementLike(postID uint64, delta int) error {
	if delta >= 0 {
		_, err := r.db.Exec(`UPDATE posts SET like_count = like_count + ? WHERE id = ?`, delta, postID)
		return err
	}
	_, err := r.db.Exec(`UPDATE posts SET like_count = GREATEST(like_count + ?, 0) WHERE id = ?`, delta, postID)
	return err
}

func (r *postRepo) IncrementComment(postID uint64, delta int) error {
	if delta >= 0 {
		_, err := r.db.Exec(`UPDATE posts SET comment_count = comment_count + ? WHERE id = ?`, delta, postID)
		return err
	}
	_, err := r.db.Exec(`UPDATE posts SET comment_count = GREATEST(comment_count + ?, 0) WHERE id = ?`, delta, postID)
	return err
}
