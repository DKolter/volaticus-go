package user

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type Handler struct {
	repo        UserRepository
	authService AuthService
}

func NewHandler(repo UserRepository, authService AuthService) *Handler {
	return &Handler{
		repo:        repo,
		authService: authService,
	}
}

type RegisterResponse struct {
	User  *User  `json:"user"`
	Token string `json:"token"`
}

func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorMsg := fmt.Sprintf("Invalid request body: %s", err.Error())
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}
	log.Printf("%s", req)
	// Basic validation
	if req.Email == "" || req.Username == "" || req.Password == "" {
		http.Error(w, "Email, username and password are required", http.StatusBadRequest)
		return
	}

	// Create user
	user, err := h.repo.Create(&req)
	if err != nil {
		switch err {
		case ErrEmailExists:
			http.Error(w, "Email already exists", http.StatusConflict)
		case ErrUsernameExists:
			http.Error(w, "Username already exists", http.StatusConflict)
		default:
			log.Panic(err.Error())
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}
	log.Printf("Created User")
	// Generate JWT token
	token, err := h.authService.GenerateToken(user)
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	// Redirect to /
	// Set JWT cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "jwt",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Only send over HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600 * 24, // 24 hours TODO: Implement token refresh
	})
	w.Header().Set("HX-Redirect", "/")
}
