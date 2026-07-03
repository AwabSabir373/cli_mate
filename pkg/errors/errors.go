package errors

import "errors"

var (
	ErrInvalidConfig = errors.New("invalid config")
	ErrUnauthorized  = errors.New("unauthorized")
	ErrToolDenied    = errors.New("tool denied")
	ErrTokenBudget   = errors.New("token budget exceeded")
)
