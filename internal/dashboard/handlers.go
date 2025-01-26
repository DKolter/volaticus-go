package dashboard

import (
	"encoding/json"
	"github.com/rs/zerolog/log"
	"net/http"
	"volaticus-go/internal/context"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) HandleGetDashboardStats(w http.ResponseWriter, r *http.Request) {
	user := context.GetUserFromContext(r.Context())
	if user == nil {
		log.Error().Msg("unauthorized access attempt to dashboard stats")
		http.Error(w, ErrUnauthorized.Error(), http.StatusUnauthorized)
		return
	}

	stats, err := h.service.GetDashboardStats(r.Context(), user.ID)
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", user.ID.String()).
			Msg("failed to fetch dashboard stats")
		http.Error(w, "Error fetching dashboard statistics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Error().
			Err(err).
			Str("user_id", user.ID.String()).
			Interface("stats", stats).
			Msg("failed to encode dashboard stats response")
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
