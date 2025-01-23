package dashboard

import "errors"

var (
	ErrFetchingStats = errors.New("error fetching dashboard statistics")
	ErrUnauthorized  = errors.New("unauthorized access to dashboard")
)
