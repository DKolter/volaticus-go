package server

import (
	"net/http"
	"volaticus-go/cmd/web/components"
	"volaticus-go/cmd/web/pages"
	"volaticus-go/internal/context"

	"github.com/a-h/templ"
	"github.com/rs/zerolog/log"
)

// Page Handlers
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	templ.Handler(pages.LoginPage()).ServeHTTP(w, r)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	templ.Handler(pages.RegisterPage()).ServeHTTP(w, r)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	templ.Handler(pages.HomePage()).ServeHTTP(w, r)
}

func (s *Server) handleUrlShort(w http.ResponseWriter, r *http.Request) {
	templ.Handler(pages.UrlShortPage()).ServeHTTP(w, r)
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	templ.Handler(pages.UploadPage()).ServeHTTP(w, r)
}

func (s *Server) handleFiles(w http.ResponseWriter, r *http.Request) {
	templ.Handler(pages.FilesPage()).ServeHTTP(w, r)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	// Get user from context
	user := context.GetUserFromContext(r.Context())
	if user == nil {
		log.Warn().
			Str("path", r.URL.Path).
			Msg("unauthorized access attempt to settings")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user's API tokens
	userTokens, err := s.authService.GetUserAPITokens(r.Context(), user.ID)
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", user.ID.String()).
			Msg("failed to fetch user tokens")
		http.Error(w, "Error fetching tokens", http.StatusInternalServerError)
		return
	}

	log.Debug().
		Str("user_id", user.ID.String()).
		Int("token_count", len(userTokens)).
		Msg("fetched user tokens")

	component := pages.SettingsPage(userTokens)
	if err := component.Render(r.Context(), w); err != nil {
		log.Error().
			Err(err).
			Str("user_id", user.ID.String()).
			Msg("failed to render settings page")
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
}

// UI Handlers
func (s *Server) showTokenModal(w http.ResponseWriter, r *http.Request) {
	user := context.GetUserFromContext(r.Context())
	if user == nil {
		log.Warn().
			Str("path", r.URL.Path).
			Msg("unauthorized access attempt to token modal")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := components.TokenModal().Render(r.Context(), w); err != nil {
		log.Error().
			Err(err).
			Str("user_id", user.ID.String()).
			Msg("failed to render token modal")
		http.Error(w, "Error rendering modal", http.StatusInternalServerError)
		return
	}
}

// API Handlers
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	health := s.db.Health(r.Context())
	s.sendJSON(w, http.StatusOK, true, "Health check successful", health)
}

// Error Handlers
func (s *Server) handleError404(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	if err := pages.Error404().Render(r.Context(), w); err != nil {
		log.Error().
			Err(err).
			Str("path", r.URL.Path).
			Msg("failed to render 404 page")
		http.Error(w, "Error rendering 404 page", http.StatusInternalServerError)
	}
}
