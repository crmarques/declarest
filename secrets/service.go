package secrets

import (
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

func NormalizePlaceholders(value resource.Value) (resource.Value, error) {
	return value, faults.ErrToBeImplemented
}
