package server

import (
	"github.com/rs/zerolog/log"
	"net/http"
	"volaticus-go/cmd/web"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth/v5"
)

func (s *Server) RegisterRoutes() http.Handler {
	r := chi.NewRouter()
	r.Use(LoggerMiddleware())
	r.Use(middleware.Recoverer)

	// JWT authentication middleware
	// Get the JWT auth instance
	tokenAuth := s.authService.GetAuth()

	if s.config.Env == "dev" || s.config.Env == "development" {
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

	// Error 404 handler
	r.NotFound(s.handleError404)

	// Public routes
	r.Group(func(r chi.Router) {
		// Login & register functionality
		r.Get("/login", s.handleLogin)
		r.Post("/login", s.userHandler.HandleLogin)
		r.Get("/register", s.handleRegister)
		r.Post("/register", s.userHandler.HandleRegister)

		// Health check
		r.Get("/health", s.healthHandler)

		// File serving and short URL redirection
		r.Get("/f/{fileUrl}", s.fileHandler.HandleServeFile)
		r.Get("/s/{shortCode}", s.shortenerHandler.HandleRedirect)
	})

	// Protected routes
	r.Group(func(r chi.Router) {
		// Add JWT verification middleware
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(s.AuthMiddleware(tokenAuth))
		r.Use(jwtauth.Authenticator(tokenAuth)) // Require authentication

		// Main pages
		r.Get("/", s.handleHome)

		// Logout
		r.Get("/logout", s.userHandler.HandleLogout)

		r.Route("/files", func(r chi.Router) {
			r.Get("/", s.handleFiles)
			r.Get("/list", s.fileHandler.HandleFilesList)
			r.Get("/stats", s.fileHandler.HandleGetFileStats)
			r.Delete("/{fileID}", s.fileHandler.HandleDeleteFile)
		})

		// Upload routes
		r.Route("/upload", func(r chi.Router) {
			r.Get("/", s.handleUpload)
			r.Post("/", s.fileHandler.HandleUpload)
			r.Post("/verify", s.fileHandler.HandleVerifyFile)
		})

		// Settings routes
		r.Route("/settings", func(r chi.Router) {
			r.Get("/", s.handleSettings)
			r.Get("/token-modal", s.showTokenModal)
			r.Post("/token-modal", s.authHandler.GenerateToken)
			r.Delete("/token/{token}", s.authHandler.DeleteToken)
		})

		// URL shortener routes
		r.Route("/url-shortener", func(r chi.Router) {
			r.Get("/", s.handleUrlShort)
			r.Get("/list", s.shortenerHandler.HandleGetUserURLs)

			r.Route("/urls", func(r chi.Router) {
				r.Post("/", s.shortenerHandler.HandleCreateShortURL)
				r.Post("/shorten", s.shortenerHandler.HandleShortenForm)
				r.Get("/{urlID}", s.shortenerHandler.HandleGetURLAnalytics)
				r.Delete("/{urlID}", s.shortenerHandler.HandleDeleteURL)
				r.Put("/{urlID}/expiration", s.shortenerHandler.HandleUpdateExpiration)
			})
		})

		// Dashboard routes
		r.Route("/dashboard", func(r chi.Router) {
			r.Get("/stats", s.dashboardHandler.HandleGetDashboardStats)
		})
	})

	// API routes with token authentication

	// API routes group
	r.Group(func(r chi.Router) {
		// All API routes will require token auth
		r.Use(s.APITokenAuthMiddleware)

		// Upload endpoint
		r.Post("/api/v1/upload", func(w http.ResponseWriter, r *http.Request) {
			log.Info().
				Str("path", r.URL.Path).
				Msg("api upload request received")
			s.fileHandler.HandleAPIUpload(w, r)
		})
	})

	return r
}
