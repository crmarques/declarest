package resource

import "github.com/crmarques/declarest/internal/support/paths"

func NormalizeLogicalPath(value string) (string, error) {
	return paths.NormalizeLogicalPath(value)
}
