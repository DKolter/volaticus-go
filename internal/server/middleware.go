package server

import (
	"log"
	"net/http"
	"strings"
	userctx "volaticus-go/internal/context"

	"github.com/go-chi/jwtauth/v5"
)

// AuthMiddleware Redirects user to /login if not authenticated, to / if authenticated
// Allows access to /login and /register without authentication
// Denys access to all other routes without authentication
func (s *Server) AuthMiddleware(ja *jwtauth.JWTAuth) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, _, err := jwtauth.FromContext(r.Context())

			// Allow files and shortened URLs without authentication
			if strings.HasPrefix(r.URL.Path, "/static/") ||
				strings.HasPrefix(r.URL.Path, "/s/") ||
				strings.HasPrefix(r.URL.Path, "/api/") ||
				strings.HasPrefix(r.URL.Path, "/f/") ||
				strings.HasSuffix(r.URL.Path, ".css") ||
				strings.HasSuffix(r.URL.Path, ".js") ||
				strings.HasSuffix(r.URL.Path, ".png") ||
				strings.HasSuffix(r.URL.Path, ".jpg") ||
				strings.HasSuffix(r.URL.Path, ".ico") {
				next.ServeHTTP(w, r)
				return
			}

			// Allow unauthenticated access to login and register
			if r.URL.Path == "/login" || r.URL.Path == "/register" {
				if err == nil && token != nil {
					// Redirect authenticated users away from login/register
					if r.Header.Get("HX-Request") == "true" {
						w.Header().Set("HX-Redirect", "/")
					} else {
						http.Redirect(w, r, "/", http.StatusSeeOther)
					}
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Require authentication for all other routes
			if err != nil || token == nil {
				if r.Header.Get("HX-Request") == "true" {
					w.Header().Set("HX-Redirect", "/login")
				} else {
					http.Redirect(w, r, "/login", http.StatusSeeOther)
				}
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// APITokenAuthMiddleware verifies API token for routes under /api/v1/
func (s *Server) APITokenAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip middleware if not an API route
		if !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}

		// Get token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		// Check Bearer token format
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization format", http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// Validate token
		apiToken, err := s.authService.ValidateAPIToken(r.Context(), token)
		if err != nil {
			log.Printf("Token validation error: %v", err)
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Get user information
		user, err := s.userService.GetByID(r.Context(), apiToken.UserID)
		if err != nil {
			log.Printf("User lookup error: %v", err)
			http.Error(w, "User not found", http.StatusUnauthorized)
			return
		}

		// Add user info to context
		userInfo := &userctx.UserInfo{
			ID:       user.ID,
			Username: user.Username,
		}
		ctx := userctx.WithUser(r.Context(), userInfo)

		// Continue with the authenticated request
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
