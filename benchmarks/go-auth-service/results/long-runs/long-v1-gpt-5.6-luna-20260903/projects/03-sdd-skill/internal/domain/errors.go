package domain

import "errors"

// ErrorKind identifies the stable class of a domain failure.
type ErrorKind string

const (
	ErrorNotFound     ErrorKind = "not_found"
	ErrorConflict     ErrorKind = "conflict"
	ErrorInvalid      ErrorKind = "invalid"
	ErrorUnauthorized ErrorKind = "unauthorized"
)

var (
	ErrNotFound     = &DomainError{Kind: ErrorNotFound}
	ErrConflict     = &DomainError{Kind: ErrorConflict}
	ErrInvalid      = &DomainError{Kind: ErrorInvalid}
	ErrUnauthorized = &DomainError{Kind: ErrorUnauthorized}
)

// DomainError is a typed error with a stable kind suitable for mapping to an
// HTTP error envelope. Details are optional and should not contain secrets.
type DomainError struct {
	Kind  ErrorKind
	Msg   string
	Cause error
}

func (e *DomainError) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return string(e.Kind)
}

func (e *DomainError) Unwrap() error { return e.Cause }

// Is lets callers use errors.Is while retaining the more specific Kind.
func (e *DomainError) Is(target error) bool {
	other, ok := target.(*DomainError)
	return ok && e.Kind == other.Kind
}

// NewError annotates a stable error kind with a safe, human-readable message.
func NewError(kind ErrorKind, message string) error {
	return &DomainError{Kind: kind, Msg: message}
}

func IsKind(err error, kind ErrorKind) bool {
	var domainErr *DomainError
	return errors.As(err, &domainErr) && domainErr.Kind == kind
}

func IsNotFound(err error) bool     { return IsKind(err, ErrorNotFound) }
func IsConflict(err error) bool     { return IsKind(err, ErrorConflict) }
func IsInvalid(err error) bool      { return IsKind(err, ErrorInvalid) }
func IsUnauthorized(err error) bool { return IsKind(err, ErrorUnauthorized) }
