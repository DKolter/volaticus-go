package user

import (
	"context"
	"database/sql"
	"errors"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/database"
)

// Repository defines the user repository interface
type Repository interface {
	// Create creates a new user
	Create(ctx context.Context, user *models.User) error
	// GetByID retrieves a user by their ID
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	// GetByEmail retrieves a user by their email
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	// GetByUsername retrieves a user by their username
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	// Update updates a user's information
	Update(ctx context.Context, user *models.User) error
	// Delete performs a soft delete of a user
	Delete(ctx context.Context, id uuid.UUID) error
}

type repository struct {
	*database.Repository
}

// NewRepository creates a new user repository
func NewRepository(db *database.DB) Repository {
	return &repository{
		Repository: database.NewRepository(db),
	}
}

func (r *repository) Create(ctx context.Context, user *models.User) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Check if email exists
		var exists bool
		if err := tx.GetContext(ctx, &exists,
			"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", user.Email); err != nil {
			return err
		}
		if exists {
			return ErrEmailExists
		}

		// Check if username exists
		if err := tx.GetContext(ctx, &exists,
			"SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)", user.Username); err != nil {
			return err
		}
		if exists {
			return ErrUsernameExists
		}

		query := `
            INSERT INTO users (id, email, username, password_hash, is_active, created_at, updated_at)
            VALUES (:id, :email, :username, :password_hash, :is_active, NOW(), NOW())`

		_, err := tx.NamedExecContext(ctx, query, user)
		return err
	})
}

func (r *repository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	err := r.Get(ctx, &user, "SELECT * FROM users WHERE id = $1", id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

func (r *repository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.Get(ctx, &user, "SELECT * FROM users WHERE email = $1", email)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

func (r *repository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	err := r.Get(ctx, &user, "SELECT * FROM users WHERE username = $1", username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrUserNotFound
	}
	return &user, err
}

func (r *repository) Update(ctx context.Context, user *models.User) error {
	return r.WithTx(ctx, func(tx *sqlx.Tx) error {
		// Check if email is being changed and ensure it's unique
		var existingUser models.User
		err := tx.GetContext(ctx, &existingUser, "SELECT email, username FROM users WHERE id = $1", user.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrUserNotFound
		}
		if err != nil {
			return err
		}

		if existingUser.Email != user.Email {
			var exists bool
			if err := tx.GetContext(ctx, &exists,
				"SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND id != $2)",
				user.Email, user.ID); err != nil {
				return err
			}
			if exists {
				return ErrEmailExists
			}
		}

		// Similar check for username
		if existingUser.Username != user.Username {
			var exists bool
			if err := tx.GetContext(ctx, &exists,
				"SELECT EXISTS(SELECT 1 FROM users WHERE username = $1 AND id != $2)",
				user.Username, user.ID); err != nil {
				return err
			}
			if exists {
				return ErrUsernameExists
			}
		}

		query := `
            UPDATE users 
            SET email = :email, 
                username = :username,
                is_active = :is_active,
                updated_at = NOW()
            WHERE id = :id`

		result, err := tx.NamedExecContext(ctx, query, user)
		if err != nil {
			return err
		}

		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows == 0 {
			return ErrUserNotFound
		}

		return nil
	})
}

func (r *repository) Delete(ctx context.Context, id uuid.UUID) error {
	result, err := r.Exec(ctx, "UPDATE users SET is_active = false, updated_at = NOW() WHERE id = $1", id)
	if err != nil {
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrUserNotFound
	}

	return nil
}
