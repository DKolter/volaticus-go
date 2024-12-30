package shortener

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// HandleCreateShortURL handles the creation of shortened URLs
func (h *Handler) HandleCreateShortURL(w http.ResponseWriter, r *http.Request) {
	var req CreateURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	response, err := h.service.CreateShortURL(req.URL)
	if err != nil {
		log.Printf("Error creating short URL: %v", err)
		http.Error(w, "Error creating short URL", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleRedirect handles the redirection from short URLs to original URLs
func (h *Handler) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "shortCode")
	if shortCode == "" {
		http.Error(w, "Short code is required", http.StatusBadRequest)
		return
	}

	originalURL, err := h.service.GetOriginalURL(shortCode)
	if err != nil {
		log.Printf("Error retrieving original URL: %v", err)
		http.Error(w, "URL not found", http.StatusNotFound)
		return
	}

	http.Redirect(w, r, originalURL, http.StatusMovedPermanently)
}

// HandleShortenForm handles the URL shortening form submission
func (h *Handler) HandleShortenForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Error parsing form", http.StatusBadRequest)
		return
	}

	url := r.FormValue("url")
	if url == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	response, err := h.service.CreateShortURL(url)
	if err != nil {
		log.Printf("Error creating short URL: %v", err)
		http.Error(w, fmt.Sprintf("Error creating short URL: %v", err), http.StatusInternalServerError)
		return
	}

	// If HTMX request, only return result component
	if r.Header.Get("HX-Request") == "true" {
		// Render result component
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div class="mt-4 p-4 bg-gray-800 rounded-lg border border-gray-700">
            <p class="text-gray-300">Your shortened URL:</p>
            <div class="mt-2 flex items-center gap-2">
                <input type="text" readonly value="%s" 
                    class="flex-1 rounded-md border-0 bg-white/5 py-1.5 text-white shadow-sm ring-1 ring-inset ring-white/10"/>
                <button onclick="navigator.clipboard.writeText('%s')" 
                    class="rounded-md bg-indigo-500 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-400">
                    Copy
                </button>
            </div>
        </div>`, response.ShortURL, response.ShortURL)
		return
	}

	// If not HTMX request, return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
