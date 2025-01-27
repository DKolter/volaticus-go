package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"volaticus-go/internal/config"
	"volaticus-go/internal/dashboard"
	"volaticus-go/internal/shortener"
	"volaticus-go/internal/storage"

	"github.com/rs/zerolog/log"

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
	storage          storage.StorageProvider
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
	// Initialize Storage
	storageProvider, err := storage.NewStorageProvider(storage.StorageConfig{
		Provider:   config.Storage.Provider,
		LocalPath:  config.Storage.LocalPath,
		BaseURL:    config.BaseURL,
		ProjectID:  config.Storage.ProjectID,
		BucketName: config.Storage.BucketName,
	})
	if err != nil {
		return nil, fmt.Errorf("initializing storage provider: %w", err)
	}
	log.Printf("Using %s storage provider", config.Storage.Provider)

	// Initialize repositories
	userRepo := user.NewRepository(db)
	tokenRepo := auth.NewRepository(db)
	fileRepo := uploader.NewRepository(db, *config)
	shortenerRepo := shortener.NewRepository(db)
	dashboardRepo := dashboard.NewRepository(db)

	// Initialize Services
	authService := auth.NewService(config.Secret, tokenRepo)
	userService := user.NewService(userRepo)
	fileService := uploader.NewService(fileRepo, config, storageProvider)
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
		storage:          storageProvider,
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
	log.Info().
		Int("port", s.config.Port).
		Str("env", s.config.Env).
		Msg("starting server")

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
		log.Error().
			Err(err).
			Interface("response", response).
			Msg("failed to encode JSON response")
	}
}

func (s *Server) Close() error {
	if err := s.storage.Close(); err != nil {
		log.Printf("Error closing storage provider: %v", err)
	}
	if err := s.db.Close(); err != nil {
		log.Printf("Error closing database connection: %v", err)
	}
	return nil
}
