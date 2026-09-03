package domain

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalid      = errors.New("invalid")
	ErrUnauthorized = errors.New("unauthorized")
	ErrTokenReuse   = errors.New("refresh token reuse")
)

// Error is a typed domain error with a stable category and a safe message.
type Error struct {
	Kind    error
	Message string
}

func (e *Error) Error() string {
	if e.Message == "" {
		return e.Kind.Error()
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Kind }

func NotFound(message string) error {
	return &Error{Kind: ErrNotFound, Message: message}
}

func Conflict(message string) error {
	return &Error{Kind: ErrConflict, Message: message}
}

func Invalid(message string) error {
	return &Error{Kind: ErrInvalid, Message: message}
}

func Unauthorized(message string) error {
	return &Error{Kind: ErrUnauthorized, Message: message}
}
