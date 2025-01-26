package user

import (
	"encoding/json"
	"errors"
	"github.com/rs/zerolog/log"
	"net/http"
	"volaticus-go/internal/common/models"
	"volaticus-go/internal/validation"
)

// AuthService defines the interface for authentication services.
// It contains a single method GenerateToken which takes a user model
// and returns a JWT token string or an error.
type AuthService interface {
	GenerateToken(user *models.User) (string, error)
}

type Handler struct {
	service     Service
	authService AuthService
}

func NewHandler(service Service, authService AuthService) *Handler {
	return &Handler{
		service:     service,
		authService: authService,
	}
}

// CreateUserRequest represents the data needed to create a new user
type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,password"`
}

// UpdateUserRequest represents the data that can be updated for a user
type UpdateUserRequest struct {
	Email    *string `json:"email" validate:"omitempty,email"`
	Username *string `json:"username" validate:"omitempty,min=3,max=50"`
	IsActive *bool   `json:"is_active"`
}

type LoginRequest struct {
	Username string `json:"username" validate:"required,username"`
	Password string `json:"password" validate:"required,min=1"`
}

func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := validation.Validate(&req); err != nil {
		errs := validation.FormatError(err)
		http.Error(w, errs[0].Error, http.StatusBadRequest)
		return
	}

	user, err := h.service.Register(r.Context(), &req)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailExists):
			http.Error(w, "Email already exists", http.StatusConflict)
		case errors.Is(err, ErrUsernameExists):
			http.Error(w, "Username already exists", http.StatusConflict)
		default:
			log.Error().
				Err(err).
				Str("username", req.Username).
				Msg("Failed to register user")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	token, err := h.authService.GenerateToken(user)
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", user.ID.String()).
			Msg("Failed to generate token")
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	// Set JWT cookie with appropriate security flags
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600 * 24, // 24 hours
	})

	// If this is a HTMX request, send a redirect
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		return
	}

	// Otherwise return success status
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := validation.Validate(&req); err != nil {
		errs := validation.FormatError(err)
		http.Error(w, errs[0].Error, http.StatusBadRequest)
		return
	}

	user, err := h.service.ValidateCredentials(r.Context(), req.Username, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrUserNotFound):
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		case errors.Is(err, ErrInvalidCredentials):
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		default:
			log.Error().
				Err(err).
				Str("username", req.Username).
				Msg("Error validating user credentials")
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	token, err := h.authService.GenerateToken(user)
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", user.ID.String()).
			Msg("Failed to generate auth token")
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600 * 24,
	})

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteStrictMode,
	})

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/login")
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
