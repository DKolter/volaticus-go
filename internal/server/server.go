package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"volaticus-go/internal/config"
	"volaticus-go/internal/shortener"

	_ "github.com/joho/godotenv/autoload"

	"volaticus-go/internal/auth"
	"volaticus-go/internal/database"
	"volaticus-go/internal/uploader"
	"volaticus-go/internal/user"
)

// Server represents the HTTP server and its dependencies
type Server struct {
	config           *config.Config
	db               database.Service
	authService      *auth.Service
	authHandler      *auth.Handler
	userHandler      *user.Handler
	fileHandler      *uploader.Handler
	shortenerHandler *shortener.Handler
}

// NewServer creates a new server instance
func NewServer(config *config.Config, db database.Service) (*Server, error) {
	// Initialize repositories
	userRepo := user.NewPostgresUserRepository(db.DB())
	tokenRepo := auth.NewPostgresTokenRepository(db.DB())
	fileRepo := uploader.NewPostgresRepository(db.DB())
	shortenerRepo := shortener.NewPostgresRepository(db.DB())

	// Initialize auth service
	authService := auth.NewService(config.Secret, tokenRepo)

	// Initialize file service & start expired files worker
	fileService := uploader.NewService(fileRepo, config)
	uploader.StartExpiredFilesWorker(fileService, 1*time.Minute)

	// Initialize shortened URL service
	shortenerService := shortener.NewService(shortenerRepo, config)

	// Initialize handlers
	userHandler := user.NewHandler(userRepo, authService)
	authHandler := auth.NewHandler(userRepo, authService)
	fileHandler := uploader.NewHandler(fileService)
	shortenerHandler := shortener.NewHandler(shortenerService)

	server := &Server{
		config:           config,
		db:               db,
		authService:      authService,
		authHandler:      authHandler,
		userHandler:      userHandler,
		fileHandler:      fileHandler,
		shortenerHandler: shortenerHandler,
	}

	return server, nil
}

// Start initializes and starts the HTTP server
func (s *Server) Start() (*http.Server, error) {
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      s.RegisterRoutes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Log server startup
	log.Printf("Starting server on port %d in %s mode", s.config.Port, s.config.Env)

	return srv, nil
}

// sendJSON sends a JSON response with consistent formatting
func (s *Server) sendJSON(w http.ResponseWriter, status int, success bool, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := APIResponse{
		Success: success,
		Message: message,
		Data:    data,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
	}
}
