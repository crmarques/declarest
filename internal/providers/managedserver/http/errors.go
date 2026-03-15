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

func notFoundError(message string, cause error) error {
	return faults.NewTypedError(faults.NotFoundError, message, cause)
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
