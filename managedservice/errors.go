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

package managedservice

import (
	"errors"

	"github.com/crmarques/declarest/faults"
)

// ListPayloadShapeError marks managed-service list decoding validation errors
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
		err: faults.Invalid(message, cause),
	}
}

func IsListPayloadShapeError(err error) bool {
	var target *ListPayloadShapeError
	return errors.As(err, &target)
}
