package auth

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"volaticus-go/internal/context"
	"volaticus-go/internal/user"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	userRepo    user.UserRepository
	authService *Service
}

func NewHandler(userRepo user.UserRepository, authService *Service) *Handler {
	return &Handler{
		userRepo:    userRepo,
		authService: authService,
	}
}

func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Get user by username
	foundUser, err := h.userRepo.GetByUsername(req.Username)
	if err != nil {
		if err == user.ErrUserNotFound {
			http.Error(w, "Invalid credentials", http.StatusUnauthorized)
			return
		}
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Validate password
	if !user.CheckPassword(req.Password, foundUser.PasswordHash) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Generate token
	token, err := h.authService.GenerateToken(foundUser)
	if err != nil {
		log.Printf("Error generating token: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	env := os.Getenv("APP_ENV")
	var https bool
	if env == "" || env == "development" {
		https = false
	}

	// Set JWT cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   https, // Only send over HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600 * 24, // 24 hours TODO: Implement token refresh
	})
	w.Header().Set("HX-Redirect", "/")
}

func (h *Handler) GenerateToken(w http.ResponseWriter, r *http.Request) {
	var req CreateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	user := context.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	token, err := h.authService.GenerateAPIToken(user.ID, req.Name)
	if err != nil {
		log.Printf("Error generating API token: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(TokenResponse{Token: token.Token, Name: token.Name, ID: token.ID}); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
	}
}

func (h *Handler) DeleteToken(w http.ResponseWriter, r *http.Request) {
	// Get token ID from URL parameters
	token := chi.URLParam(r, "token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	// Get current user from context
	user := context.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Delete token, ensuring it belongs to current user
	err := h.authService.repo.DeleteTokenByUserIdAndToken(user.ID, token)
	if err != nil {
		log.Printf("failed to delete token: %v", err)
		http.Error(w, "failed to delete token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Refresh", "true")

	// Return success with no content
	w.WriteHeader(http.StatusNoContent)
}
