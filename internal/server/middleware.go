package server

import (
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"net/http"
	"strings"
	"time"
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
			log.Error().
				Err(err).
				Str("token", token).
				Msg("token validation failed")
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Get user information
		user, err := s.userService.GetByID(r.Context(), apiToken.UserID)
		if err != nil {
			log.Error().
				Err(err).
				Str("user_id", apiToken.UserID.String()).
				Msg("user lookup failed")
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

// LoggerMiddleware logs request details and duration
func LoggerMiddleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip noisy static asset logging
			if isStaticAsset(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Generate or get request ID
			requestID := middleware.GetReqID(r.Context())
			if requestID == "" {
				requestID = uuid.New().String()[:8]
			}

			// Group logs by request using consistent fields
			reqLogger := log.With().
				Str("rid", requestID).
				Str("method", r.Method).
				Str("path", shortenPath(r.URL.Path)). // Shorten very long paths
				Logger()

			// Initial request log
			reqLogger.Info().
				Str("ip", anonymizeIP(r.RemoteAddr)).         // Anonymize IPs in logs for privacy
				Str("ua", summarizeUserAgent(r.UserAgent())). // Summarize UA
				Msg("Request started")

			defer func() {
				duration := time.Since(start)

				// Detailed completion log
				event := reqLogger.Info()
				if ww.Status() >= 400 {
					event = reqLogger.Error()
				}

				event.
					Int("status", ww.Status()).
					Str("duration", formatDuration(duration)).
					Str("size", humanize.Bytes(uint64(ww.BytesWritten()))).
					Msg("Request completed")
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

// Helper functions for cleaner logging

func isStaticAsset(path string) bool {
	return strings.HasPrefix(path, "/assets/") ||
		strings.HasPrefix(path, "/static/") ||
		strings.HasSuffix(path, ".css") ||
		strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".ico") ||
		strings.HasSuffix(path, ".png") ||
		strings.HasSuffix(path, ".jpg")
}

func shortenPath(path string) string {
	if len(path) > 50 {
		return path[:20] + "..." + path[len(path)-20:]
	}
	return path
}

func anonymizeIP(ip string) string {
	parts := strings.Split(ip, ":")
	if len(parts) > 0 {
		if parts[0] == "[::1]" {
			return "localhost"
		}
		// Anonymize last octet for IPv4
		ipParts := strings.Split(parts[0], ".")
		if len(ipParts) == 4 {
			ipParts[3] = "xxx"
			return strings.Join(ipParts, ".")
		}
	}
	return ip
}

func summarizeUserAgent(ua string) string {
	if strings.Contains(ua, "curl") {
		return "curl"
	}
	if len(ua) > 50 {
		return ua[:47] + "..."
	}
	return ua
}

func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%dµs", d.Microseconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}
