package server

import (
	"errors"

	"github.com/crmarques/declarest/faults"
)

// ListPayloadShapeError marks resource-server list decoding validation errors
// whose shape semantics callers may treat specially for fallback behavior.
type ListPayloadShapeError struct {
	err error
}

func (e *ListPayloadShapeError) Error() string {
	if e == nil || e.err == nil {
		return "<nil>"
	}
	return e.err.Error()
}

func (e *ListPayloadShapeError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func NewListPayloadShapeError(message string, cause error) error {
	return &ListPayloadShapeError{
		err: faults.NewTypedError(faults.ValidationError, message, cause),
	}
}

func IsListPayloadShapeError(err error) bool {
	var target *ListPayloadShapeError
	return errors.As(err, &target)
}
