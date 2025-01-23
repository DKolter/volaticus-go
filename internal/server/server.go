package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"volaticus-go/internal/config"
	"volaticus-go/internal/dashboard"
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
	db               *database.DB
	authService      auth.Service
	userService      user.Service
	authHandler      *auth.Handler
	userHandler      *user.Handler
	fileHandler      *uploader.Handler
	shortenerHandler *shortener.Handler
	dashboardHandler *dashboard.Handler
}

// NewServer creates a new server instance
func NewServer(config *config.Config, db *database.DB) (*Server, error) {
	// Initialize repositories
	userRepo := user.NewRepository(db)
	tokenRepo := auth.NewRepository(db)
	fileRepo := uploader.NewRepository(db)
	shortenerRepo := shortener.NewRepository(db)
	dashboardRepo := dashboard.NewRepository(db)

	// Initialize Services
	authService := auth.NewService(config.Secret, tokenRepo)
	userService := user.NewService(userRepo)
	fileService := uploader.NewService(fileRepo, config)
	dashboardService := dashboard.NewService(dashboardRepo)

	// Initialize file service & start expired files worker
	ctx := context.Background() // TODO: Use proper context
	uploader.StartExpiredFilesWorker(ctx, fileService, 1*time.Minute)

	// Initialize shortened URL service
	shortenerService := shortener.NewService(shortenerRepo, config)

	// Initialize handlers
	userHandler := user.NewHandler(userService, authService)
	authHandler := auth.NewHandler(userRepo, authService)
	fileHandler := uploader.NewHandler(fileService)
	shortenerHandler := shortener.NewHandler(shortenerService)
	dashboardHandler := dashboard.NewHandler(dashboardService)

	server := &Server{
		config:           config,
		db:               db,
		authService:      authService,
		userService:      userService,
		authHandler:      authHandler,
		userHandler:      userHandler,
		fileHandler:      fileHandler,
		shortenerHandler: shortenerHandler,
		dashboardHandler: dashboardHandler,
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
