package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
	"volaticus-go/cmd/web/components"
	"volaticus-go/cmd/web/pages"
	"volaticus-go/internal/shortener"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"volaticus-go/internal/context"
)

type Handler struct {
	service *shortener.Service
}

func NewHandler(service *shortener.Service) *Handler {
	return &Handler{
		service: service,
	}
}

// HandleCreateShortURL handles the creation of shortened URLs via API
func (h *Handler) HandleCreateShortURL(w http.ResponseWriter, r *http.Request) {
	var req shortener.CreateURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		shortener.HandleError(w, &shortener.APIError{
			Code:    shortener.ErrCodeInvalidInput,
			Message: "Invalid request body",
		}, http.StatusBadRequest)
		return
	}

	user := context.GetUserFromContext(r.Context())
	if user == nil {
		shortener.HandleError(w, shortener.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	response, err := h.service.CreateShortURL(user.ID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "vanity code") {
			shortener.HandleError(w, shortener.ErrVanityCodeTaken, http.StatusConflict)
			return
		}
		log.Printf("Error creating short URL: %v", err)
		shortener.HandleError(w, shortener.LogError(err, "creating short URL"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}

// HandleRedirect handles the redirection and analytics recording
func (h *Handler) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "shortCode")
	if shortCode == "" {
		shortener.HandleError(w, &shortener.APIError{
			Code:    shortener.ErrCodeInvalidInput,
			Message: "Short code is required",
		}, http.StatusBadRequest)
		return
	}

	// Gather request information for analytics
	reqInfo := &shortener.RequestInfo{
		Referrer:  r.Referer(),
		UserAgent: r.UserAgent(),
		IPAddress: getIPAddress(r),
	}

	originalURL, err := h.service.GetOriginalURL(shortCode, reqInfo)
	if err != nil {
		if strings.Contains(err.Error(), "expired") {
			shortener.HandleError(w, shortener.ErrURLExpired, http.StatusGone)
			return
		}
		log.Printf("Error retrieving original URL: %v", err)
		shortener.HandleError(w, shortener.ErrURLNotFound, http.StatusNotFound)
		return
	}

	http.Redirect(w, r, originalURL, http.StatusTemporaryRedirect)
}

func (h *Handler) HandleGetUserURLs(w http.ResponseWriter, r *http.Request) {
	user := context.GetUserFromContext(r.Context())
	if user == nil {
		shortener.HandleError(w, shortener.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	urls, err := h.service.GetUserURLs(user.ID)
	if err != nil {
		log.Printf("Error retrieving user URLs: %v", err)
		shortener.HandleError(w, shortener.LogError(err, "retrieving user URLs"), http.StatusInternalServerError)
		return
	}

	// Render the template using the pages package
	if err := pages.URLList(urls).Render(r.Context(), w); err != nil {
		log.Printf("Error rendering URL list: %v", err)
		shortener.HandleError(w, shortener.LogError(err, "rendering URL list"), http.StatusInternalServerError)
	}
}

// HandleGetURLAnalytics returns analytics for a specific URL
func (h *Handler) HandleGetURLAnalytics(w http.ResponseWriter, r *http.Request) {
	urlID, err := uuid.Parse(chi.URLParam(r, "urlID"))
	if err != nil {
		shortener.HandleError(w, &shortener.APIError{
			Code:    shortener.ErrCodeInvalidInput,
			Message: "Invalid URL ID",
		}, http.StatusBadRequest)
		return
	}

	user := context.GetUserFromContext(r.Context())
	if user == nil {
		shortener.HandleError(w, shortener.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	analytics, err := h.service.GetURLAnalytics(urlID, user.ID)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			shortener.HandleError(w, shortener.ErrUnauthorized, http.StatusForbidden)
			return
		}
		log.Printf("Error retrieving URL analytics: %v", err)
		shortener.HandleError(w, shortener.LogError(err, "retrieving analytics"), http.StatusInternalServerError)
		return
	}

	// If HTMX request, return the modal template
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		if err := components.AnalyticsModal(analytics).Render(r.Context(), w); err != nil {
			log.Printf("Error rendering analytics modal: %v", err)
			shortener.HandleError(w, shortener.LogError(err, "rendering analytics modal"), http.StatusInternalServerError)
		}
		return
	}

	// Otherwise return JSON
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(analytics)
	if err != nil {
		return
	}
}

func (h *Handler) HandleDeleteURL(w http.ResponseWriter, r *http.Request) {
	urlID := chi.URLParam(r, "urlID")
	if urlID == "" {
		shortener.HandleError(w, &shortener.APIError{
			Code:    shortener.ErrCodeInvalidInput,
			Message: "URL ID is required",
		}, http.StatusBadRequest)
		return
	}

	user := context.GetUserFromContext(r.Context())
	if user == nil {
		shortener.HandleError(w, shortener.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	// Check if the URL ID is a valid UUID
	if _, err := uuid.Parse(urlID); err != nil {
		// Handle non-UUID short codes
		if err := h.service.DeleteURLByShortCode(urlID, user.ID); err != nil {
			if strings.Contains(err.Error(), "unauthorized") {
				shortener.HandleError(w, shortener.ErrUnauthorized, http.StatusForbidden)
				return
			}
			log.Printf("Error deleting short code: %v", err)
			shortener.HandleError(w, shortener.LogError(err, "deleting short code"), http.StatusInternalServerError)
			return
		}
	} else {
		// Handle UUIDs
		if err := h.service.DeleteURL(uuid.MustParse(urlID), user.ID); err != nil {
			if strings.Contains(err.Error(), "unauthorized") {
				shortener.HandleError(w, shortener.ErrUnauthorized, http.StatusForbidden)
				return
			}
			log.Printf("Error deleting URL: %v", err)
			shortener.HandleError(w, shortener.LogError(err, "deleting URL"), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Trigger", "urlsChanged")
	w.WriteHeader(http.StatusNoContent)
}

// HandleUpdateExpiration handles updating the URL expiration
func (h *Handler) HandleUpdateExpiration(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		shortener.HandleError(w, &shortener.APIError{
			Code:    shortener.ErrCodeInvalidInput,
			Message: "Error parsing form",
		}, http.StatusBadRequest)
		return
	}

	urlID, err := uuid.Parse(chi.URLParam(r, "urlID"))
	if err != nil {
		shortener.HandleError(w, &shortener.APIError{
			Code:    shortener.ErrCodeInvalidInput,
			Message: "Invalid URL ID",
		}, http.StatusBadRequest)
		return
	}

	user := context.GetUserFromContext(r.Context())
	if user == nil {
		shortener.HandleError(w, shortener.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	var expiresAt *time.Time
	if expStr := r.FormValue("expires_at"); expStr != "" {
		// Parse the local time string
		expTime, err := time.ParseInLocation("2006-01-02T15:04", expStr, time.Local)
		if err != nil {
			shortener.HandleError(w, &shortener.APIError{
				Code:    shortener.ErrCodeInvalidInput,
				Message: "Invalid expiration date format",
			}, http.StatusBadRequest)
			return
		}
		expiresAt = &expTime
	}

	if err := h.service.UpdateURLExpiration(urlID, user.ID, expiresAt); err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			shortener.HandleError(w, shortener.ErrUnauthorized, http.StatusForbidden)
			return
		}
		log.Printf("Error updating URL expiration: %v", err)
		shortener.HandleError(w, shortener.LogError(err, "updating expiration"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", "urlsChanged")
	w.WriteHeader(http.StatusNoContent)
}

// HandleShortenForm handles the URL shortening form submission with HTML response
func (h *Handler) HandleShortenForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		shortener.HandleError(w, &shortener.APIError{
			Code:    shortener.ErrCodeInvalidInput,
			Message: "Error parsing form",
		}, http.StatusBadRequest)
		return
	}

	user := context.GetUserFromContext(r.Context())
	if user == nil {
		shortener.HandleError(w, shortener.ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	req := shortener.CreateURLRequest{
		URL:        r.FormValue("url"),
		VanityCode: r.FormValue("vanity_code"),
	}

	if expStr := r.FormValue("expires_at"); expStr != "" {
		expTime, err := time.ParseInLocation("2006-01-02T15:04", expStr, time.Local)
		if err != nil {
			shortener.HandleError(w, &shortener.APIError{
				Code:    shortener.ErrCodeInvalidInput,
				Message: "Invalid expiration date format",
				Details: err.Error(),
			}, http.StatusBadRequest)
			return
		}
		req.ExpiresAt = &expTime
	}

	response, err := h.service.CreateShortURL(user.ID, &req)
	if err != nil {
		log.Printf("Error creating short URL: %v", err)

		// If HTMX request, return the shortened URL template
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html")
			errorMessage := "Error creating shortened URL"

			if strings.Contains(err.Error(), "between 4 and 30") {
				errorMessage = "Custom URL must be between 4 and 30 characters"
			} else if strings.Contains(err.Error(), "already in use") {
				errorMessage = "This custom URL is already taken"
			}

			err := pages.ErrorResult(errorMessage).Render(r.Context(), w)
			if err != nil {
				return
			}
			return
		}

		shortener.HandleError(w, shortener.LogError(err, "creating short URL"), http.StatusInternalServerError)
		return
	}

	// If HTMX request, return the shortened URL template
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("HX-Trigger", "urlsChanged")
		err := pages.ShortenedURLResult(response).Render(r.Context(), w)
		if err != nil {
			return
		}
		return
	}

	// If not HTMX, return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("HX-Trigger", "urlsChanged")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}

// Helper functions

// getIPAddress gets the client's IP address
func getIPAddress(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	host := strings.Split(r.RemoteAddr, ":")[0]
	if host == "[" || host == "[]" || host == "[::1]" || host == "" {
		return "127.0.0.1" // Return localhost IP for development
	}
	return host
}
