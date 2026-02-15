package common

import (
	"github.com/crmarques/declarest/faults"
)

func ValidationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}
