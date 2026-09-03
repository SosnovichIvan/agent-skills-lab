package domain

import "errors"

var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalid      = errors.New("invalid")
	ErrUnauthorized = errors.New("unauthorized")
)

// Error preserves a stable domain error category while allowing callers to
// attach a safe, human-readable message and an underlying cause.
type Error struct {
	Kind    error
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Kind.Error()
}

func (e *Error) Unwrap() error { return e.Cause }

func (e *Error) Is(target error) bool { return target == e.Kind }

func NewNotFound(message string) error     { return newError(ErrNotFound, message) }
func NewConflict(message string) error     { return newError(ErrConflict, message) }
func NewInvalid(message string) error      { return newError(ErrInvalid, message) }
func NewUnauthorized(message string) error { return newError(ErrUnauthorized, message) }

func Wrap(kind, message string, cause error) error {
	category := ErrInvalid
	switch kind {
	case "not_found":
		category = ErrNotFound
	case "conflict":
		category = ErrConflict
	case "unauthorized":
		category = ErrUnauthorized
	}
	return &Error{Kind: category, Message: message, Cause: cause}
}

func newError(kind error, message string) error { return &Error{Kind: kind, Message: message} }

// IsKind reports whether err belongs to the requested domain category.
func IsKind(err, kind error) bool { return errors.Is(err, kind) }
