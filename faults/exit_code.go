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

func ExitCodeForError(err error) int {
	if err == nil {
		return 0
	}

	var typedErr *TypedError
	if !errors.As(err, &typedErr) {
		return 1
	}

	switch typedErr.Category {
	case ValidationError:
		return 2
	case NotFoundError:
		return 3
	case AuthError:
		return 4
	case ConflictError:
		return 5
	case TransportError:
		return 6
	default:
		return 1
	}
}
