package auth

import (
	"encoding/json"
	"net/http"
	"volaticus-go/internal/context"
	"volaticus-go/internal/user"
	"volaticus-go/internal/validation"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	userRepo    user.Repository
	authService Service
}

type CreateTokenRequest struct {
	Name   string    `json:"name" validate:"required"`
	UserID uuid.UUID `json:"userid" validate:"required"`
}

type TokenResponse struct {
	Token string    `json:"token"`
	Name  string    `json:"name"`
	ID    uuid.UUID `json:"id"`
}

func NewHandler(userRepo user.Repository, authService Service) *Handler {
	return &Handler{
		userRepo:    userRepo,
		authService: authService,
	}
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
	req.UserID = user.ID

	if err := validation.Validate(&req); err != nil {
		errors := validation.FormatError(err)
		log.Error().
			Interface("errors", errors).
			Msg("Validation errors")
		http.Error(w, errors[0].Error, http.StatusBadRequest)
		return
	}

	token, err := h.authService.GenerateAPIToken(r.Context(), user.ID, req.Name)
	if err != nil {
		log.Error().
			Err(err).
			Msg("Error generating API token")
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(TokenResponse{Token: token.Token, Name: token.Name, ID: token.ID}); err != nil {
		log.Error().
			Err(err).
			Msg("Error encoding response")
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
	err := h.authService.DeleteTokenByUserIdAndToken(r.Context(), user.ID, token)
	if err != nil {
		log.Error().
			Err(err).
			Str("token", token).
			Str("user_id", user.ID.String()).
			Msg("Failed to delete token")
		http.Error(w, "failed to delete token", http.StatusInternalServerError)
		return
	}

	// Return success for htmx-delete request
	w.WriteHeader(http.StatusOK)
}
