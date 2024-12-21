package context

import (
	"context"
	"log"

	"github.com/go-chi/jwtauth/v5"
	"github.com/google/uuid"
)

type contextKey string

const (
	userContextKey contextKey = "user"
)

type UserInfo struct {
	ID       uuid.UUID
	Username string
	// Additional fields from JWT claims can be added here
}

// GetUserFromContext retrieves user info from context
func GetUserFromContext(ctx context.Context) *UserInfo {
	_, claims, err := jwtauth.FromContext(ctx)
	if err != nil {
		return nil
	}

	// If already stored in context, return it
	if user, ok := ctx.Value(userContextKey).(*UserInfo); ok {
		return user
	}

	// Otherwise parse from JWT claims
	return getUserFromClaims(claims)
}

// getUserFromClaims creates UserInfo from JWT claims
func getUserFromClaims(claims map[string]interface{}) *UserInfo {
	userID, _ := claims["user_id"].(string)
	username, _ := claims["username"].(string)
	log.Printf("Got user claims from JWt: %s : %s", username, userID)
	if userID == "" || username == "" {
		return nil
	}
	parsedId, err := uuid.Parse(userID)
	if err != nil {
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
