package repository

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/instagram-clone/user-service/internal/models"
)

type UserRepository interface {
	Create(user *models.User) (*models.User, error)
	FindByID(id uint64) (*models.User, error)
	FindByUsername(username string) (*models.User, error)
	FindByEmail(email string) (*models.User, error)
	FindByUsernameOrEmail(usernameOrEmail string) (*models.User, error)
	Update(id uint64, updates map[string]interface{}) error
	UpdatePassword(id uint64, hash string) error
	SoftDelete(id uint64) error
	Search(query string, page, pageSize int) ([]*models.User, int64, error)
	ExistsByUsername(username string) (bool, error)
	ExistsByEmail(email string) (bool, error)
}

type userRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{db: db}
}

const userColumns = `id, username, email, password_hash, full_name, bio, avatar_url, 
	website, is_private, is_verified, is_active, role, 
	post_count, follower_count, following_count, created_at, updated_at`

func scanUser(row interface{ Scan(...interface{}) error }) (*models.User, error) {
	u := &models.User{}
	err := row.Scan(
		&u.ID, &u.Username, &u.Email, &u.PasswordHash,
		&u.FullName, &u.Bio, &u.AvatarURL, &u.Website,
		&u.IsPrivate, &u.IsVerified, &u.IsActive, &u.Role,
		&u.PostCount, &u.FollowerCount, &u.FollowingCount,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (r *userRepository) Create(user *models.User) (*models.User, error) {
	query := `INSERT INTO users 
		(username, email, password_hash, full_name, bio, avatar_url, role)
		VALUES (?, ?, ?, ?, '', '', 'user')`

	result, err := r.db.Exec(query, user.Username, user.Email, user.PasswordHash, user.FullName)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	id, _ := result.LastInsertId()
	return r.FindByID(uint64(id))
}

func (r *userRepository) FindByID(id uint64) (*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE id = ? AND deleted_at IS NULL`
	row := r.db.QueryRow(query, id)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *userRepository) FindByUsername(username string) (*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE username = ? AND deleted_at IS NULL`
	row := r.db.QueryRow(query, username)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *userRepository) FindByEmail(email string) (*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE email = ? AND deleted_at IS NULL`
	row := r.db.QueryRow(query, email)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *userRepository) FindByUsernameOrEmail(val string) (*models.User, error) {
	query := `SELECT ` + userColumns + ` FROM users 
		WHERE (username = ? OR email = ?) AND deleted_at IS NULL LIMIT 1`
	row := r.db.QueryRow(query, val, val)
	u, err := scanUser(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return u, err
}

func (r *userRepository) Update(id uint64, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	setClauses := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates)+1)

	for col, val := range updates {
		setClauses = append(setClauses, col+" = ?")
		args = append(args, val)
	}
	args = append(args, id)

	query := fmt.Sprintf("UPDATE users SET %s WHERE id = ?", strings.Join(setClauses, ", "))
	_, err := r.db.Exec(query, args...)
	return err
}

func (r *userRepository) UpdatePassword(id uint64, hash string) error {
	_, err := r.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

func (r *userRepository) SoftDelete(id uint64) error {
	_, err := r.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (r *userRepository) Search(query string, page, pageSize int) ([]*models.User, int64, error) {
	like := "%" + query + "%"
	offset := (page - 1) * pageSize

	var total int64
	countQ := `SELECT COUNT(*) FROM users WHERE (username LIKE ? OR full_name LIKE ?) AND deleted_at IS NULL AND is_active = TRUE`
	if err := r.db.QueryRow(countQ, like, like).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(
		`SELECT `+userColumns+` FROM users 
		WHERE (username LIKE ? OR full_name LIKE ?) AND deleted_at IS NULL AND is_active = TRUE
		ORDER BY follower_count DESC LIMIT ? OFFSET ?`,
		like, like, pageSize, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	return users, total, nil
}

func (r *userRepository) ExistsByUsername(username string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE username = ? AND deleted_at IS NULL`, username).Scan(&count)
	return count > 0, err
}

func (r *userRepository) ExistsByEmail(email string) (bool, error) {
	var count int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM users WHERE email = ? AND deleted_at IS NULL`, email).Scan(&count)
	return count > 0, err
}
