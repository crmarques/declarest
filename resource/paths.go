package resource

import (
	"path"
	"strings"

	"github.com/crmarques/declarest/faults"
)

type RawPathParseOptions struct {
	AllowMissingLeadingSlash bool
}

type ParsedRawPath struct {
	Raw                      string
	Normalized               string
	Segments                 []string
	ExplicitCollectionTarget bool
}

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

func ParseRawPathWithOptions(value string, options RawPathParseOptions) (ParsedRawPath, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ParsedRawPath{}, faults.NewTypedError(faults.ValidationError, "logical path must not be empty", nil)
	}

	normalizedInput := strings.ReplaceAll(trimmed, "\\", "/")
	explicitCollectionTarget := normalizedInput != "/" && strings.HasSuffix(normalizedInput, "/")
	if options.AllowMissingLeadingSlash && !strings.HasPrefix(normalizedInput, "/") {
		normalizedInput = "/" + normalizedInput
	}

	normalized, err := CleanRawPath(normalizedInput)
	if err != nil {
		return ParsedRawPath{}, err
	}
	if normalized == "/" {
		explicitCollectionTarget = false
	}

	return ParsedRawPath{
		Raw:                      trimmed,
		Normalized:               normalized,
		Segments:                 SplitRawPathSegments(normalized),
		ExplicitCollectionTarget: explicitCollectionTarget,
	}, nil
}

func HasExplicitCollectionTarget(value string) bool {
	parsed, err := ParseRawPathWithOptions(value, RawPathParseOptions{})
	if err != nil {
		return false
	}
	return parsed.ExplicitCollectionTarget
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

func HasLogicalPathOverlap(a string, b string) (bool, error) {
	left, err := NormalizeLogicalPath(a)
	if err != nil {
		return false, err
	}
	right, err := NormalizeLogicalPath(b)
	if err != nil {
		return false, err
	}

	if left == right {
		return true, nil
	}
	if strings.HasPrefix(left, right) && overlapBoundaryMatch(left, right) {
		return true, nil
	}
	if strings.HasPrefix(right, left) && overlapBoundaryMatch(right, left) {
		return true, nil
	}
	return false, nil
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

func ValidateLogicalPathSegment(segment string) error {
	_, err := normalizeLogicalPathSegment(segment)
	return err
}

// SplitRawPathSegments splits a path string into its segments without
// validation. Use SplitLogicalPathSegments when the path should be validated
// first (rejects reserved segments like "_").
func SplitRawPathSegments(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return nil
	}

	segments := make([]string, 0, strings.Count(trimmed, "/")+1)
	var current strings.Builder
	templateDepth := 0

	for idx := 0; idx < len(trimmed); idx++ {
		if idx+1 < len(trimmed) && trimmed[idx] == '{' && trimmed[idx+1] == '{' {
			templateDepth++
			current.WriteString("{{")
			idx++
			continue
		}
		if idx+1 < len(trimmed) && trimmed[idx] == '}' && trimmed[idx+1] == '}' && templateDepth > 0 {
			templateDepth--
			current.WriteString("}}")
			idx++
			continue
		}
		if trimmed[idx] == '/' && templateDepth == 0 {
			segments = append(segments, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(trimmed[idx])
	}

	if current.Len() > 0 {
		segments = append(segments, current.String())
	}

	return segments
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

func overlapBoundaryMatch(candidate string, prefix string) bool {
	if prefix == "/" {
		return true
	}
	if len(candidate) <= len(prefix) {
		return false
	}
	return candidate[len(prefix)] == '/'
}

func normalizeLogicalPathSegment(segment string) (string, error) {
	trimmedSegment := strings.TrimSpace(segment)
	if trimmedSegment == "" {
		return "", faults.NewTypedError(faults.ValidationError, "logical path segment must not be empty", nil)
	}
	if trimmedSegment == "." || trimmedSegment == ".." {
		return "", faults.NewTypedError(faults.ValidationError, "logical path segment must not contain traversal segments", nil)
	}
	if trimmedSegment == "_" {
		return "", faults.NewTypedError(faults.ValidationError, "logical path segment must not contain reserved metadata segment \"_\"", nil)
	}
	if strings.Contains(trimmedSegment, "/") || strings.Contains(trimmedSegment, "\\") {
		return "", faults.NewTypedError(faults.ValidationError, "logical path segment must not contain path separators", nil)
	}
	return trimmedSegment, nil
}
