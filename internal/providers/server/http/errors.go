package http

import "github.com/crmarques/declarest/faults"

const (
	ValidationError = faults.ValidationError
	NotFoundError   = faults.NotFoundError
	ConflictError   = faults.ConflictError
	AuthError       = faults.AuthError
	TransportError  = faults.TransportError
	InternalError   = faults.InternalError
)

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}

func notFoundError(message string, cause error) error {
	return faults.NewTypedError(faults.NotFoundError, message, cause)
}

func conflictError(message string, cause error) error {
	return faults.NewTypedError(faults.ConflictError, message, cause)
}

func authError(message string, cause error) error {
	return faults.NewTypedError(faults.AuthError, message, cause)
}

func transportError(message string, cause error) error {
	return faults.NewTypedError(faults.TransportError, message, cause)
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}

func isTypedCategory(err error, category faults.ErrorCategory) bool {
	return faults.IsCategory(err, category)
}
