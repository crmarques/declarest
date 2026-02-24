package resource

import "github.com/crmarques/declarest/faults"

func isTypedErrorCategory(err error, category faults.ErrorCategory) bool {
	return faults.IsCategory(err, category)
}
