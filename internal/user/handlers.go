package user

import (
	"encoding/json"
	"log"
	"net/http"
)

type Handler struct {
	service     *Service
	authService AuthService
}

func NewHandler(repo UserRepository, authService AuthService) *Handler {
	return &Handler{
		service:     NewService(repo),
		authService: authService,
	}
}

// CreateUserRequest represents the data needed to create a new user
type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3,max=50"`
	Password string `json:"password" validate:"required,min=8"`
}

// UpdateUserRequest represents the data that can be updated for a user
type UpdateUserRequest struct {
	Email    *string `json:"email" validate:"omitempty,email"`
	Username *string `json:"username" validate:"omitempty,min=3,max=50"`
	IsActive *bool   `json:"is_active"`
}

func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create user using service
	user, err := h.service.RegisterUser(&req)
	if err != nil {
		switch err {
		case ErrEmailExists:
			http.Error(w, "Email already exists", http.StatusConflict)
		case ErrUsernameExists:
			http.Error(w, "Username already exists", http.StatusConflict)
		case ErrInvalidInput:
			http.Error(w, "Invalid input", http.StatusBadRequest)
		default:
			log.Printf("Error registering user: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// Generate JWT token
	token, err := h.authService.GenerateToken(user)
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	// Set JWT cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600 * 24,
	})
	//w.Header().Set("HX-Redirect", "/")
}
