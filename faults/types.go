// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

func NewValidationError(message string, cause error) *TypedError {
	return NewTypedError(ValidationError, message, cause)
}

func NewConflictError(message string, cause error) *TypedError {
	return NewTypedError(ConflictError, message, cause)
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
