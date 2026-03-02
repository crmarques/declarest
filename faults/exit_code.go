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
