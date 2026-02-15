package notimpl

import "github.com/crmarques/declarest/faults"

func Error(typeName string, method string) error {
	return faults.NewTypedError(
		faults.InternalError,
		typeName+"."+method+" not implemented",
		faults.ErrToBeImplemented,
	)
}
