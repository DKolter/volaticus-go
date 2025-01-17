package server

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth/v5"
	"net/http"
	"volaticus-go/cmd/web"
	"volaticus-go/internal/shortener"
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
		r.Get("/f/{fileUrl}", s.fileHandler.HandleServeFile)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		// Pages
		r.Get("/", s.handleHome)
		r.Get("/url-short", s.handleUrlShort)
		r.Get("/upload", s.handleUpload)
		r.Post("/upload/verify", s.fileHandler.HandleVerifyFile)
		r.Post("/upload", s.fileHandler.HandleUpload)
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
			// r.Post("/upload", s.handleFileUpload)
		})
	})

	// URL Shortener routes
	shortenerService := shortener.NewService(
		shortener.NewPostgresRepository(s.db.DB()),
		fmt.Sprintf("%s:%d", s.config.BaseURL, s.config.Port), // Use config BaseURL
	)
	shortenerHandler := shortener.NewHandler(shortenerService)

	// Frontend routes
	r.Route("/url-short", func(r chi.Router) {
		r.Get("/", s.handleUrlShort)                       // Main page
		r.Get("/list", shortenerHandler.HandleGetUserURLs) // Get user's URLs
	})

	// API routes for URL shortener
	r.Route("/api/urls", func(r chi.Router) {
		r.Post("/", shortenerHandler.HandleCreateShortURL)     // Create via API
		r.Post("/shorten", shortenerHandler.HandleShortenForm) // Create via form
		r.Get("/{urlID}", shortenerHandler.HandleGetURLAnalytics)
		r.Delete("/{urlID}", shortenerHandler.HandleDeleteURL)
		r.Put("/{urlID}/expiration", shortenerHandler.HandleUpdateExpiration)
	})

	// Public route for redirecting
	r.Get("/s/{shortCode}", shortenerHandler.HandleRedirect)

	return r
}
