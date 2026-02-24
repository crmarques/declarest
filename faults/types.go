package faults

import "errors"

type ErrorCategory string

const (
	ValidationError ErrorCategory = "ValidationError"
	NotFoundError   ErrorCategory = "NotFoundError"
	ConflictError   ErrorCategory = "ConflictError"
	AuthError       ErrorCategory = "AuthError"
	TransportError  ErrorCategory = "TransportError"
	InternalError   ErrorCategory = "InternalError"
)

type TypedError struct {
	Category ErrorCategory
	Message  string
	Cause    error
}

func (e *TypedError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Message != "" && e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return string(e.Category)
}

func (e *TypedError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func NewTypedError(category ErrorCategory, message string, cause error) *TypedError {
	return &TypedError{
		Category: category,
		Message:  message,
		Cause:    cause,
	}
}

func IsCategory(err error, category ErrorCategory) bool {
	if err == nil {
		return false
	}

	var typedErr *TypedError
	if !errors.As(err, &typedErr) {
		return false
	}
	return typedErr.Category == category
}
