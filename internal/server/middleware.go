package server

import (
	"net/http"
	"strings"

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
