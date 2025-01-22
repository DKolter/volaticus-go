package uploader

import (
	"errors"
)

var (
	ErrDuplicateURLValue = errors.New("duplicate URL value")
	ErrNoRows            = errors.New("no rows found")
	ErrTransaction       = errors.New("transaction error")
	ErrCommit            = errors.New("commit transaction error")
	ErrRollback          = errors.New("rollback transaction error")
	ErrNoFile            = errors.New("no file provided")
	ErrFileTooLarge      = errors.New("file exceeds maximum allowed size")
	ErrInvalidURLType    = errors.New("invalid URL type")
	ErrUnauthorized      = errors.New("unauthorized")
)
