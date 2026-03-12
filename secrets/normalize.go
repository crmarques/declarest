package secrets

import (
	"strings"

	"github.com/crmarques/declarest/faults"
)

func NormalizeKey(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" {
		return "", faults.NewValidationError("secret key must not be empty", nil)
	}

	parts := strings.Split(trimmed, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", faults.NewValidationError("secret key contains invalid path segment", nil)
		}
	}

	return strings.Join(parts, "/"), nil
}
