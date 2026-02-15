package common

import (
	"fmt"

	"github.com/crmarques/declarest/faults"
)

func ValidationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}

func NotImplementedError(component string, operation string) error {
	return faults.NewTypedError(
		faults.InternalError,
		fmt.Sprintf("%s.%s not implemented", component, operation),
		faults.ErrToBeImplemented,
	)
}
