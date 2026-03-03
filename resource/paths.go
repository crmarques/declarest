package resource

import (
	"path"
	"strings"

	"github.com/crmarques/declarest/faults"
)

// CleanRawPath normalizes an absolute path by replacing backslashes, rejecting
// traversal segments, and cleaning redundant separators. Unlike
// NormalizeLogicalPath it does NOT reject reserved segments like "_", so it is
// suitable for paths that may contain wildcards or metadata placeholders.
func CleanRawPath(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", faults.NewTypedError(faults.ValidationError, "logical path must not be empty", nil)
	}

	normalizedInput := strings.ReplaceAll(value, "\\", "/")
	if !strings.HasPrefix(normalizedInput, "/") {
		return "", faults.NewTypedError(faults.ValidationError, "logical path must be absolute", nil)
	}

	for _, segment := range strings.Split(normalizedInput, "/") {
		if segment == ".." {
			return "", faults.NewTypedError(faults.ValidationError, "logical path must not contain traversal segments", nil)
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

func NormalizeLogicalPath(value string) (string, error) {
	cleaned, err := CleanRawPath(value)
	if err != nil {
		return "", err
	}

	for _, segment := range SplitRawPathSegments(cleaned) {
		if segment == "_" {
			return "", faults.NewTypedError(faults.ValidationError, "logical path must not contain reserved metadata segment \"_\"", nil)
		}
	}

	return cleaned, nil
}

func JoinLogicalPath(collectionPath string, segment string) (string, error) {
	trimmedSegment := strings.TrimSpace(segment)
	if trimmedSegment == "" {
		return "", faults.NewTypedError(faults.ValidationError, "logical path segment must not be empty", nil)
	}

	joined := path.Join(collectionPath, trimmedSegment)
	if !strings.HasPrefix(joined, "/") {
		joined = "/" + joined
	}

	return NormalizeLogicalPath(joined)
}

// SplitRawPathSegments splits a path string into its segments without
// validation. Use SplitLogicalPathSegments when the path should be validated
// first (rejects reserved segments like "_").
func SplitRawPathSegments(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func SplitLogicalPathSegments(value string) []string {
	normalized, err := NormalizeLogicalPath(value)
	if err != nil || normalized == "/" {
		return nil
	}
	return strings.Split(strings.TrimPrefix(normalized, "/"), "/")
}

func ChildSegment(parentPath string, candidatePath string) (string, bool) {
	normalizedParentPath, err := NormalizeLogicalPath(parentPath)
	if err != nil {
		return "", false
	}
	normalizedCandidatePath, err := NormalizeLogicalPath(candidatePath)
	if err != nil {
		return "", false
	}

	if normalizedParentPath == "/" {
		remaining := strings.TrimPrefix(normalizedCandidatePath, "/")
		if remaining == "" || strings.Contains(remaining, "/") {
			return "", false
		}
		return remaining, true
	}

	parentPrefix := strings.TrimSuffix(normalizedParentPath, "/")
	if !strings.HasPrefix(normalizedCandidatePath, parentPrefix+"/") {
		return "", false
	}

	remaining := strings.TrimPrefix(normalizedCandidatePath, parentPrefix+"/")
	if remaining == "" || strings.Contains(remaining, "/") {
		return "", false
	}

	return remaining, true
}
