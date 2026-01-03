package resource

import (
	"fmt"
	"strings"
)

func ValidateLogicalPath(path string) error {
	return validatePath(path, false)
}

func ValidateMetadataPath(path string) error {
	return validatePath(path, true)
}

func validatePath(path string, allowWildcards bool) error {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return fmt.Errorf("path is required")
	}
	if !strings.HasPrefix(trimmed, "/") {
		return fmt.Errorf("path must start with /")
	}
	if strings.Contains(trimmed, "\\") {
		return fmt.Errorf("path must use / as separator")
	}
	if strings.Contains(trimmed, "//") {
		return fmt.Errorf("path must not contain empty segments")
	}
	if trimmed == "/" {
		return nil
	}

	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	for _, segment := range segments {
		if segment == "" {
			return fmt.Errorf("path must not contain empty segments")
		}
		if segment == ".." {
			return fmt.Errorf("path must not contain .. segments")
		}
		if !allowWildcards && segment == "_" {
			return fmt.Errorf("path must not contain reserved '_' segments")
		}
	}
	return nil
}

func SplitPathSegments(path string) []string {
	trimmed := strings.Trim(path, " /")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	var segments []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments = append(segments, part)
	}
	return segments
}
