package user

import (
	"context"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"
	"volaticus-go/internal/common/models"
)

type Service interface {
	Register(ctx context.Context, req *CreateUserRequest) (*models.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	ValidateCredentials(ctx context.Context, username, password string) (*models.User, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) Register(ctx context.Context, req *CreateUserRequest) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to hash password")
		return nil, err
	}

	user := &models.User{
		ID:           uuid.New(),
		Email:        req.Email,
		Username:     req.Username,
		PasswordHash: string(hash),
		IsActive:     true,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		log.Error().
			Err(err).
			Str("username", user.Username).
			Msg("Failed to create user")
		return nil, err
	}

	log.Info().
		Str("user_id", user.ID.String()).
		Str("username", user.Username).
		Msg("New user registered")
	return user, nil
}

func (s *service) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *service) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.repo.GetByEmail(ctx, email)
}

func (s *service) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	return s.repo.GetByUsername(ctx, username)
}

func (s *service) ValidateCredentials(ctx context.Context, username, password string) (*models.User, error) {
	user, err := s.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		log.Info().
			Str("username", username).
			Msg("Failed login attempt")
		return nil, ErrInvalidCredentials
	}

	log.Info().
		Str("user_id", user.ID.String()).
		Str("username", user.Username).
		Msg("User logged in successfully")
	return user, nil
}

func (s *service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		log.Error().
			Err(err).
			Str("user_id", id.String()).
			Msg("Failed to delete user")
		return err
	}

	log.Info().
		Str("user_id", id.String()).
		Msg("User deleted")
	return nil
}
