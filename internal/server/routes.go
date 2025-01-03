package server

import (
	"fmt"
	"net/http"
	"volaticus-go/cmd/web"
	"volaticus-go/internal/shortener"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth/v5"
)

// TODO: Move this into config / get from env variables
const (
	MaxFileSize = 100 << 20 // 100 MB
	UploadDir   = "./tmp"
)

func (s *Server) RegisterRoutes() http.Handler {

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// JWT authentication middleware
	// Get the JWT auth instance
	tokenAuth := s.authService.GetAuth()

	// Add JWT verification middleware
	r.Use(jwtauth.Verifier(tokenAuth))
	r.Use(s.AuthMiddleware(tokenAuth))

	if s.config.Env == "development" {
		r.Use(middleware.NoCache)
	}

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Serve static files
	fileServer := http.FileServer(http.FS(web.Files)) // embedded in binary
	r.Handle("/assets/*", fileServer)

	// Public routes
	r.Group(func(r chi.Router) {
		r.Get("/login", s.handleLogin)
		r.Post("/login", s.authHandler.HandleLogin)
		r.Get("/register", s.handleRegister)
		r.Post("/register", s.userHandler.HandleRegister)
		r.Get("/health", s.healthHandler)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		// Pages
		r.Get("/", s.handleHome)
		r.Get("/url-short", s.handleUrlShort)
		r.Get("/upload", s.handleUpload)
		r.Get("/files", s.handleFiles)
		r.Get("/settings", s.handleSettings)
		r.Get("/settings/token-modal", s.showTokenModal)
		r.Post("/settings/token-modal", s.authHandler.GenerateToken)
		r.Delete("/settings/token/{token}", s.authHandler.DeleteToken)
	})

	// API routes
	r.Group(func(r chi.Router) {
		// API versioning
		r.Route("/v1", func(r chi.Router) {
			// TODO: add middleware to application token / auth middleware
			r.Post("/upload", s.handleFileUpload)
		})
	})

	// URL Shortener routes
	shortenerService := shortener.NewService(
		shortener.NewPostgresRepository(s.db.DB()),
		fmt.Sprintf("http://localhost:%d", s.config.Port), // TODO: get from config
	)
	shortenerHandler := shortener.NewHandler(shortenerService)

	// Frontend route
	r.Get("/url-short", s.handleUrlShort)

	// API/Form routes
	r.Post("/url-short/shorten", shortenerHandler.HandleShortenForm)
	r.Get("/{shortCode}", shortenerHandler.HandleRedirect)

	return r
}
