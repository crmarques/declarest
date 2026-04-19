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

func NotFound(message string, cause error) *TypedError {
	return NewTypedError(NotFoundError, message, cause)
}

func Invalid(message string, cause error) *TypedError {
	return NewTypedError(ValidationError, message, cause)
}

func Conflict(message string, cause error) *TypedError {
	return NewTypedError(ConflictError, message, cause)
}

func Auth(message string, cause error) *TypedError {
	return NewTypedError(AuthError, message, cause)
}

func Transport(message string, cause error) *TypedError {
	return NewTypedError(TransportError, message, cause)
}

func Internal(message string, cause error) *TypedError {
	return NewTypedError(InternalError, message, cause)
}
