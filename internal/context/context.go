package context

import (
	"context"
	"github.com/go-chi/jwtauth/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type contextKey string

const (
	userContextKey contextKey = "user"
)

type UserInfo struct {
	ID       uuid.UUID
	Username string
}

// GetUserFromContext retrieves user info from context, handling both direct context and JWT
func GetUserFromContext(ctx context.Context) *UserInfo {
	// check for direct user info set by API token auth middleware
	if user, ok := ctx.Value(userContextKey).(*UserInfo); ok {
		return user
	}

	// Check JWT claims as fallback for session auth for web handlers
	_, claims, err := jwtauth.FromContext(ctx)
	if err != nil {
		log.Debug().Err(err).Msg("no JWT found in context")
		return nil
	}

	userID, _ := claims["user_id"].(string)
	username, _ := claims["username"].(string)
	if userID == "" || username == "" {
		log.Debug().
			Str("user_id", userID).
			Str("username", username).
			Msg("incomplete user information in JWT claims")
		return nil
	}

	parsedId, err := uuid.Parse(userID)
	if err != nil {
		log.Error().
			Err(err).
			Str("user_id", userID).
			Msg("failed to parse user ID from JWT claims")
		return nil
	}

	return &UserInfo{
		ID:       parsedId,
		Username: username,
	}
}

// WithUser adds user info to the context
func WithUser(ctx context.Context, user *UserInfo) context.Context {
	return context.WithValue(ctx, userContextKey, user)
}
