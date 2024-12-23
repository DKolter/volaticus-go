package server

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"volaticus-go/cmd/web/components"
	"volaticus-go/cmd/web/pages"
	"volaticus-go/internal/context"

	"github.com/a-h/templ"
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
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get user's API tokens
	userTokens, err := s.authService.GetUserAPITokens(user.ID)
	if err != nil {
		// Log error but don't expose internal error to user
		log.Printf("Error fetching user tokens: %v", err)
		http.Error(w, "Error fetching tokens", http.StatusInternalServerError)
		return
	}
	log.Printf("Found %d tokens for User %s", len(userTokens), user.Username)

	// Render settings page with tokens
	component := pages.SettingsPage(userTokens)
	if err := component.Render(r.Context(), w); err != nil {
		log.Printf("Error rendering settings page: %v", err)
		http.Error(w, "Error rendering page", http.StatusInternalServerError)
		return
	}
}

// UI Handlers
func (s *Server) showTokenModal(w http.ResponseWriter, r *http.Request) {
	user := context.GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if err := components.TokenModal().Render(r.Context(), w); err != nil {
		log.Printf("Error rendering token modal: %v", err)
		http.Error(w, "Error rendering modal", http.StatusInternalServerError)
		return
	}
}

// API Handlers
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	health := s.db.Health()
	s.sendJSON(w, http.StatusOK, true, "Health check successful", health)
}

func (s *Server) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxFileSize)

	if err := r.ParseMultipartForm(MaxFileSize); err != nil {
		s.sendJSON(w, http.StatusRequestEntityTooLarge, false, "File too large", nil)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		s.sendJSON(w, http.StatusBadRequest, false, "Invalid file", nil)
		return
	}
	defer file.Close()

	if err := os.MkdirAll(UploadDir, 0755); err != nil {
		log.Printf("Error creating upload directory: %v", err)
		s.sendJSON(w, http.StatusInternalServerError, false, "Failed to create upload directory", nil)
		return
	}

	dst, err := os.Create(filepath.Join(UploadDir, header.Filename))
	if err != nil {
		log.Printf("Error creating file: %v", err)
		s.sendJSON(w, http.StatusInternalServerError, false, "Failed to create file", nil)
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		log.Printf("Error saving file: %v", err)
		s.sendJSON(w, http.StatusInternalServerError, false, "Failed to save file", nil)
		return
	}

	s.sendJSON(w, http.StatusOK, true, "File uploaded successfully", FileUploadData{
		Filename: header.Filename,
		Size:     header.Size,
	})
}
