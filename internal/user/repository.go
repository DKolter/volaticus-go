package user

import (
	"database/sql"
	"fmt"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"volaticus-go/internal/common/models"
)

// UserRepository defines methods for user persistence
type UserRepository interface {
	Create(user *CreateUserRequest) (*models.User, error)
	GetByID(id uuid.UUID) (*models.User, error)
	GetByEmail(email string) (*models.User, error)
	GetByUsername(username string) (*models.User, error)
	Update(id uuid.UUID, updates *UpdateUserRequest) error
	Delete(id uuid.UUID) error
}

type postgresUserRepository struct {
	db *sqlx.DB
}

// NewPostgresUserRepository creates a new PostgreSQL user repository
func NewPostgresUserRepository(db *sqlx.DB) UserRepository {
	return &postgresUserRepository{db: db}
}

func (r *postgresUserRepository) Create(req *CreateUserRequest) (*models.User, error) {
	// Check if email exists
	var exists bool
	err := r.db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", req.Email)
	if err != nil {
		return nil, fmt.Errorf("checking email existence: %w", err)
	}
	if exists {
		return nil, ErrEmailExists
	}

	// Check if username exists
	err = r.db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", req.Username)
	if err != nil {
		return nil, fmt.Errorf("checking username existence: %w", err)
	}
	if exists {
		return nil, ErrUsernameExists
	}

	// Hash password
	passwordHash, err := HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: passwordHash,
		IsActive:     true,
	}

	query := `
        INSERT INTO users (id, email, username, password_hash, is_active)
        VALUES ($1, $2, $3, $4, $5)
        RETURNING *`

	err = r.db.Get(user, query,
		user.ID, user.Email, user.Username, user.PasswordHash, user.IsActive)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	return user, nil
}

func (r *postgresUserRepository) GetByID(id uuid.UUID) (*models.User, error) {
	user := new(models.User)
	err := r.db.Get(user, "SELECT * FROM users WHERE id = $1", id)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by id: %w", err)
	}
	return user, nil
}

func (r *postgresUserRepository) GetByEmail(email string) (*models.User, error) {
	user := new(models.User)
	err := r.db.Get(user, "SELECT * FROM users WHERE email = $1", email)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by email: %w", err)
	}
	return user, nil
}

func (r *postgresUserRepository) GetByUsername(username string) (*models.User, error) {
	user := new(models.User)
	err := r.db.Get(user, "SELECT * FROM users WHERE username = $1", username)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("getting user by username: %w", err)
	}
	return user, nil
}

func (r *postgresUserRepository) Update(id uuid.UUID, req *UpdateUserRequest) error {
	query := `
        UPDATE users
        SET email = COALESCE($1, email),
            username = COALESCE($2, username),
            is_active = COALESCE($3, is_active),
            updated_at = CURRENT_TIMESTAMP
        WHERE id = $4`

	var email, username *string
	var isActive *bool

	if req != nil {
		// Check if email exists if it's being updated
		if req.Email != nil {
			var exists bool
			err := r.db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND id != $2)", req.Email, id)
			if err != nil {
				return fmt.Errorf("checking email existence: %w", err)
			}
			if exists {
				return ErrEmailExists
			}
			email = req.Email
		}

		// Check if username exists if it's being updated
		if req.Username != nil {
			var exists bool
			err := r.db.Get(&exists, "SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 AND id != $2)", req.Username, id)
			if err != nil {
				return fmt.Errorf("checking username existence: %w", err)
			}
			if exists {
				return ErrUsernameExists
			}
			username = req.Username
		}

		isActive = req.IsActive
	}

	result, err := r.db.Exec(query, email, username, isActive, id)
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}

func (r *postgresUserRepository) Delete(id uuid.UUID) error {
	result, err := r.db.Exec("DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("getting rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrUserNotFound
	}

	return nil
}
