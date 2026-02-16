package paths

import (
	"path"
	"strings"

	"github.com/crmarques/declarest/faults"
)

func NormalizeLogicalPath(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", faults.NewTypedError(faults.ValidationError, "logical path must not be empty", nil)
	}

	normalizedInput := strings.ReplaceAll(value, "\\", "/")
	if !strings.HasPrefix(normalizedInput, "/") {
		return "", faults.NewTypedError(faults.ValidationError, "logical path must be absolute", nil)
	}

	segments := strings.Split(normalizedInput, "/")
	for _, segment := range segments {
		if segment == ".." {
			return "", faults.NewTypedError(faults.ValidationError, "logical path must not contain traversal segments", nil)
		}
		if segment == "_" {
			return "", faults.NewTypedError(faults.ValidationError, "logical path must not contain reserved metadata segment \"_\"", nil)
		}
	}

	cleaned := path.Clean(normalizedInput)
	if !strings.HasPrefix(cleaned, "/") {
		return "", faults.NewTypedError(faults.ValidationError, "logical path must be absolute", nil)
	}

	if cleaned != "/" {
		cleaned = strings.TrimSuffix(cleaned, "/")
	}

	return cleaned, nil
}
