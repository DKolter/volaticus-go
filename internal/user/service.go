package user

import (
	"fmt"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"volaticus-go/internal/common/models"
)

// HashPassword creates a bcrypt hash from a password string
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword checks if the provided password matches the hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

type AuthService interface {
	GenerateToken(user *models.User) (string, error)
}

type Service struct {
	repo UserRepository
}

func NewService(repo UserRepository) *Service {
	return &Service{
		repo: repo,
	}
}

// RegisterUser handles the business logic for user registration
func (s *Service) RegisterUser(req *CreateUserRequest) (*models.User, error) {
	// Validate input
	if req.Email == "" || req.Username == "" || req.Password == "" {
		return nil, ErrInvalidInput
	}

	// Create user
	user, err := s.repo.Create(req)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	return user, nil
}

// GetUserByID retrieves a user by their ID
func (s *Service) GetUserByID(id uuid.UUID) (*models.User, error) {
	user, err := s.repo.GetByID(id)
	if err != nil {
		return nil, fmt.Errorf("getting user: %w", err)
	}
	return user, nil
}

// UpdateUser updates user information
func (s *Service) UpdateUser(id uuid.UUID, req *UpdateUserRequest) error {
	return s.repo.Update(id, req)
}

// DeleteUser deletes a user account
func (s *Service) DeleteUser(id uuid.UUID) error {
	return s.repo.Delete(id)
}

// ValidateCredentials checks if the provided credentials are valid
func (s *Service) ValidateCredentials(username, password string) (*models.User, error) {
	user, err := s.repo.GetByUsername(username)
	if err != nil {
		if err == ErrUserNotFound {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !CheckPassword(password, user.PasswordHash) {
		return nil, ErrInvalidCredentials
	}

	return user, nil
}
