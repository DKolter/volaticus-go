package auth

import "errors"

var (
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenExists   = errors.New("token already exists")
	ErrTokenRevoked  = errors.New("token is revoked")
	ErrTokenExpired  = errors.New("token has expired")
)
