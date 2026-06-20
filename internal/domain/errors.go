package domain

import "errors"

var (
	// ErrNotFound is returned when a requested resource does not exist in the store.
	ErrNotFound = errors.New("not found")
	// ErrUnauthorized is returned when the caller's token is absent or invalid.
	ErrUnauthorized = errors.New("unauthorized")
	// ErrInvalidInput is returned when the caller provides structurally invalid data.
	ErrInvalidInput = errors.New("invalid input")
)
