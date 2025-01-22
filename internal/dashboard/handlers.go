package dashboard

import (
	"encoding/json"
	"log"
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
		http.Error(w, ErrUnauthorized.Error(), http.StatusUnauthorized)
		return
	}

	stats, err := h.service.GetDashboardStats(r.Context(), user.ID)
	if err != nil {
		log.Printf("Error fetching dashboard stats: %v", err)
		http.Error(w, "Error fetching dashboard statistics", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		log.Printf("Error encoding dashboard stats: %v", err)
		http.Error(w, "Error encoding response", http.StatusInternalServerError)
		return
	}
}
