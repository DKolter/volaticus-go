package shortener

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"volaticus-go/internal/context"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// HandleCreateShortURL handles the creation of shortened URLs via API
func (h *Handler) HandleCreateShortURL(w http.ResponseWriter, r *http.Request) {
	var req CreateURLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		HandleError(w, &APIError{
			Code:    ErrCodeInvalidInput,
			Message: "Invalid request body",
		}, http.StatusBadRequest)
		return
	}

	user := context.GetUserFromContext(r.Context())
	if user == nil {
		HandleError(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	response, err := h.service.CreateShortURL(user.ID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "vanity code") {
			HandleError(w, ErrVanityCodeTaken, http.StatusConflict)
			return
		}
		log.Printf("Error creating short URL: %v", err)
		HandleError(w, LogError(err, "creating short URL"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HandleRedirect handles the redirection and analytics recording
func (h *Handler) HandleRedirect(w http.ResponseWriter, r *http.Request) {
	shortCode := chi.URLParam(r, "shortCode")
	if shortCode == "" {
		HandleError(w, &APIError{
			Code:    ErrCodeInvalidInput,
			Message: "Short code is required",
		}, http.StatusBadRequest)
		return
	}

	// Gather request information for analytics
	reqInfo := &RequestInfo{
		Referrer:  r.Referer(),
		UserAgent: r.UserAgent(),
		IPAddress: getIPAddress(r),
	}

	originalURL, err := h.service.GetOriginalURL(shortCode, reqInfo)
	if err != nil {
		if strings.Contains(err.Error(), "expired") {
			HandleError(w, ErrURLExpired, http.StatusGone)
			return
		}
		log.Printf("Error retrieving original URL: %v", err)
		HandleError(w, ErrURLNotFound, http.StatusNotFound)
		return
	}

	http.Redirect(w, r, originalURL, http.StatusMovedPermanently)
}

func (h *Handler) HandleGetUserURLs(w http.ResponseWriter, r *http.Request) {
	user := context.GetUserFromContext(r.Context())
	if user == nil {
		HandleError(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	urls, err := h.service.GetUserURLs(user.ID)
	if err != nil {
		log.Printf("Error retrieving user URLs: %v", err)
		HandleError(w, LogError(err, "retrieving user URLs"), http.StatusInternalServerError)
		return
	}

	// If HTMX request, return HTML response
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `<div class="space-y-4">`)
		for _, url := range urls {
			expiryText := "Never"
			if url.ExpiresAt != nil {
				expiryText = url.ExpiresAt.Format("2006-01-02 15:04")
			}

			fmt.Fprintf(w, `
                <div class="bg-gray-800 p-4 rounded-lg border border-gray-700">
                    <div class="flex flex-col space-y-2">
                        <div class="flex justify-between items-center">
                            <div class="flex items-center">
                                <a href="/%s" class="text-indigo-400 hover:text-indigo-300 mr-2" target="_blank">/%s</a>
                                <button
                                    onclick="copyToClipboard('%s')"
                                    class="text-gray-400 hover:text-gray-300"
                                    title="Copy to clipboard"
                                >
                                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"></path>
                                    </svg>
                                </button>
                                <button
                                    onclick="showQRCode('%s')"
                                    class="text-gray-400 hover:text-gray-300 ml-2"
                                    title="Show QR Code"
                                >
                                    <svg class="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4v1m6 11h2m-6 0h-2v4m0-11v3m0 0h.01M12 12h4.01M16 20h4M4 12h4m12 0h.01M5 8h2a1 1 0 001-1V5a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1zm12 0h2a1 1 0 001-1V5a1 1 0 00-1-1h-2a1 1 0 00-1 1v2a1 1 0 001 1zM5 20h2a1 1 0 001-1v-2a1 1 0 00-1-1H5a1 1 0 00-1 1v2a1 1 0 001 1z"></path>
                                    </svg>
                                </button>
                            </div>
                            <div class="flex items-center gap-4">
    							<div class="text-sm text-gray-400">
        						Expires: %s
    							</div>
    							<div class="text-sm text-gray-400">
    							    %d clicks
    							</div>
    							<button 
    							    hx-get="/api/urls/%s"
        							hx-target="#analytics-modal"
        							class="text-indigo-400 hover:text-indigo-300">
        							Analytics
    							</button>
    							<button 
        							hx-delete="/api/urls/%s"
        							hx-confirm="Are you sure you want to delete this URL?"
        							class="text-red-400 hover:text-red-300">
        							Delete
    							</button>
							</div>
                        </div>
                        <div class="text-sm text-gray-400 break-all">
                            %s
                        </div>
                    </div>
                </div>
            `, url.ShortCode, url.ShortCode, url.ShortCode, url.ShortCode, expiryText, url.AccessCount, url.ID, url.ShortCode, url.OriginalURL)
		}
		fmt.Fprintf(w, `</div>

        <script>
            // Copy to clipboard function
            function copyToClipboard(shortCode) {
                const url = window.location.origin + "/" + shortCode;
                navigator.clipboard.writeText(url).then(() => {
                    showToast('URL copied to clipboard');
                });
            }

            // QR Code function
            function showQRCode(shortCode) {
                const url = window.location.origin + "/" + shortCode;
                const qrModal = document.createElement('div');
                qrModal.className = 'fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50';
                qrModal.innerHTML = `+"`"+`
                    <div class="bg-gray-800 rounded-lg p-6 max-w-sm w-full mx-4">
                        <div class="flex justify-between items-center mb-4">
                            <h3 class="text-xl font-semibold text-white">QR Code</h3>
                            <button onclick="this.closest('.fixed').remove()" class="text-gray-400 hover:text-white">
                                <svg class="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path>
                                </svg>
                            </button>
                        </div>
                        <div class="flex justify-center">
                            <img src="https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=${encodeURIComponent(url)}" 
                                 alt="QR Code" 
                                 class="w-48 h-48 bg-white p-2 rounded"
                            />
                        </div>
                        <p class="mt-4 text-sm text-center text-gray-400">
                            Scan this code to open the shortened URL
                        </p>
                    </div>
                `+"`"+`;
                document.body.appendChild(qrModal);
            }

            // Toast notification function
            function showToast(message) {
                const toast = document.createElement('div');
                toast.className = 'fixed bottom-4 right-4 bg-gray-800 text-white px-4 py-2 rounded shadow-lg transition-opacity duration-300';
                toast.textContent = message;
                document.body.appendChild(toast);
                setTimeout(() => {
                    toast.style.opacity = '0';
                    setTimeout(() => {
                        toast.remove();
                    }, 300);
                }, 3000);
            }
        </script>`)
		return
	}

	// If not HTMX request, return JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(urls)
}

// HandleGetURLAnalytics returns analytics for a specific URL
func (h *Handler) HandleGetURLAnalytics(w http.ResponseWriter, r *http.Request) {
	urlID, err := uuid.Parse(chi.URLParam(r, "urlID"))
	if err != nil {
		HandleError(w, &APIError{
			Code:    ErrCodeInvalidInput,
			Message: "Invalid URL ID",
		}, http.StatusBadRequest)
		return
	}

	user := context.GetUserFromContext(r.Context())
	if user == nil {
		HandleError(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	analytics, err := h.service.GetURLAnalytics(urlID, user.ID)
	if err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			HandleError(w, ErrUnauthorized, http.StatusForbidden)
			return
		}
		log.Printf("Error retrieving URL analytics: %v", err)
		HandleError(w, LogError(err, "retrieving analytics"), http.StatusInternalServerError)
		return
	}

	// If HTMX request, return the modal template
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		if err := RenderAnalyticsModal(analytics).Render(r.Context(), w); err != nil {
			log.Printf("Error rendering analytics modal: %v", err)
			HandleError(w, LogError(err, "rendering analytics modal"), http.StatusInternalServerError)
		}
		return
	}

	// Otherwise return JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analytics)
}

func (h *Handler) HandleDeleteURL(w http.ResponseWriter, r *http.Request) {
	urlID := chi.URLParam(r, "urlID")
	if urlID == "" {
		HandleError(w, &APIError{
			Code:    ErrCodeInvalidInput,
			Message: "URL ID is required",
		}, http.StatusBadRequest)
		return
	}

	user := context.GetUserFromContext(r.Context())
	if user == nil {
		HandleError(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	// Check if the URL ID is a valid UUID
	if _, err := uuid.Parse(urlID); err != nil {
		// Handle non-UUID short codes
		if err := h.service.DeleteURLByShortCode(urlID, user.ID); err != nil {
			if strings.Contains(err.Error(), "unauthorized") {
				HandleError(w, ErrUnauthorized, http.StatusForbidden)
				return
			}
			log.Printf("Error deleting short code: %v", err)
			HandleError(w, LogError(err, "deleting short code"), http.StatusInternalServerError)
			return
		}
	} else {
		// Handle UUIDs
		if err := h.service.DeleteURL(uuid.MustParse(urlID), user.ID); err != nil {
			if strings.Contains(err.Error(), "unauthorized") {
				HandleError(w, ErrUnauthorized, http.StatusForbidden)
				return
			}
			log.Printf("Error deleting URL: %v", err)
			HandleError(w, LogError(err, "deleting URL"), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("HX-Trigger", "urlsChanged")
	w.WriteHeader(http.StatusNoContent)
}

// HandleUpdateExpiration handles updating the URL expiration
func (h *Handler) HandleUpdateExpiration(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		HandleError(w, &APIError{
			Code:    ErrCodeInvalidInput,
			Message: "Error parsing form",
		}, http.StatusBadRequest)
		return
	}

	urlID, err := uuid.Parse(chi.URLParam(r, "urlID"))
	if err != nil {
		HandleError(w, &APIError{
			Code:    ErrCodeInvalidInput,
			Message: "Invalid URL ID",
		}, http.StatusBadRequest)
		return
	}

	user := context.GetUserFromContext(r.Context())
	if user == nil {
		HandleError(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	var expiresAt *time.Time
	if expStr := r.FormValue("expires_at"); expStr != "" {
		// Parse the local time string
		expTime, err := time.ParseInLocation("2006-01-02T15:04", expStr, time.Local)
		if err != nil {
			HandleError(w, &APIError{
				Code:    ErrCodeInvalidInput,
				Message: "Invalid expiration date format",
			}, http.StatusBadRequest)
			return
		}
		expiresAt = &expTime
	}

	if err := h.service.UpdateURLExpiration(urlID, user.ID, expiresAt); err != nil {
		if strings.Contains(err.Error(), "unauthorized") {
			HandleError(w, ErrUnauthorized, http.StatusForbidden)
			return
		}
		log.Printf("Error updating URL expiration: %v", err)
		HandleError(w, LogError(err, "updating expiration"), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", "urlsChanged")
	w.WriteHeader(http.StatusNoContent)
}

// HandleShortenForm handles the URL shortening form submission with HTML response
func (h *Handler) HandleShortenForm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		HandleError(w, &APIError{
			Code:    ErrCodeInvalidInput,
			Message: "Error parsing form",
		}, http.StatusBadRequest)
		return
	}

	user := context.GetUserFromContext(r.Context())
	if user == nil {
		HandleError(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	req := CreateURLRequest{
		URL:        r.FormValue("url"),
		VanityCode: r.FormValue("vanity_code"),
	}

	// Parse expiration if provided
	if expStr := r.FormValue("expires_at"); expStr != "" {
		// Parse the local time string
		expTime, err := time.ParseInLocation("2006-01-02T15:04", expStr, time.Local)
		if err != nil {
			HandleError(w, &APIError{
				Code:    ErrCodeInvalidInput,
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

		// If HTMX request, return error message
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("Content-Type", "text/html")
			var errorMessage string

			switch {
			case strings.Contains(err.Error(), "between 4 and 30"):
				errorMessage = "Custom URL must be between 4 and 30 characters"
			case strings.Contains(err.Error(), "already in use"):
				errorMessage = "This custom URL is already taken"
			default:
				errorMessage = "Error creating shortened URL"
			}

			fmt.Fprintf(w, `
            <div class="mt-4">
                <p class="text-red-400">%s</p>
            </div>
        `, errorMessage)
			return
		}

		HandleError(w, LogError(err, "creating short URL"), http.StatusInternalServerError)
		return
	}

	// If HTMX request, return result component
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("HX-Trigger", "urlsChanged")
		fmt.Fprintf(w, `
            <div class="mt-4 p-4 bg-gray-800 rounded-lg border border-gray-700">
                <p class="text-gray-300">Your shortened URL:</p>
                <div class="mt-2 flex items-center gap-2">
                    <input type="text" readonly value="%s" 
                        class="flex-1 rounded-md border-0 bg-white/5 py-1.5 text-white shadow-sm ring-1 ring-inset ring-white/10"/>
                    <button onclick="navigator.clipboard.writeText('%s')" 
                        class="rounded-md bg-indigo-500 px-3 py-2 text-sm font-semibold text-white shadow-sm hover:bg-indigo-400">
                        Copy
                    </button>
                </div>
                %s
            </div>`,
			response.ShortURL,
			response.ShortURL,
			h.getExpirationMessage(response.ExpiresAt),
		)
		return
	}

	// If not HTMX request, return JSON response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("HX-Trigger", "urlsChanged")
	json.NewEncoder(w).Encode(response)
}

// Helper functions

// getExpirationMessage formats the expiration message
func (h *Handler) getExpirationMessage(expiresAt *time.Time) string {
	if expiresAt == nil {
		return ""
	}
	return fmt.Sprintf(`
        <p class="mt-2 text-sm text-gray-400">
            This URL will expire on %s
        </p>`,
		expiresAt.Format("January 2, 2006 at 15:04 MST"),
	)
}

// getIPAddress gets the client's IP address
func getIPAddress(r *http.Request) string {
	// Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}
	// Fall back to RemoteAddr
	return strings.Split(r.RemoteAddr, ":")[0]
}
