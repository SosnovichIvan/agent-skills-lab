package domain

import "fmt"

// ErrorKind classifies errors at the application boundary.
type ErrorKind string

const (
	KindNotFound     ErrorKind = "not_found"
	KindConflict     ErrorKind = "conflict"
	KindInvalid      ErrorKind = "invalid"
	KindUnauthorized ErrorKind = "unauthorized"
)

// Error is a typed domain error. The optional cause is retained for logging
// and inspection without losing the stable classification.
type Error struct {
	Kind    ErrorKind
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return string(e.Kind)
}

func (e *Error) Unwrap() error { return e.Cause }

func (e *Error) Is(target error) bool {
	other, ok := target.(*Error)
	return ok && e.Kind == other.Kind
}

var (
	ErrNotFound     = &Error{Kind: KindNotFound}
	ErrConflict     = &Error{Kind: KindConflict}
	ErrInvalid      = &Error{Kind: KindInvalid}
	ErrUnauthorized = &Error{Kind: KindUnauthorized}
)

func NewError(kind ErrorKind, message string) error {
	return &Error{Kind: kind, Message: message}
}

func WrapError(kind ErrorKind, message string, cause error) error {
	if cause == nil {
		return NewError(kind, message)
	}
	return &Error{Kind: kind, Message: message, Cause: cause}
}

func (e *Error) Format(state fmt.State, verb rune) {
	fmt.Fprintf(state, "%s", e.Error())
}
