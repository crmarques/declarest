package metadata

import (
	"path"
	"strings"

	"github.com/crmarques/declarest/faults"
)

type PathDescriptor struct {
	Selector     string
	Segments     []string
	Collection   bool
	SelectorMode bool
}

func ParsePathDescriptor(logicalPath string) (PathDescriptor, error) {
	trimmed := strings.TrimSpace(logicalPath)
	if trimmed == "" {
		return PathDescriptor{}, faults.NewTypedError(
			faults.ValidationError,
			"logical path must not be empty",
			nil,
		)
	}

	normalizedInput := strings.ReplaceAll(trimmed, "\\", "/")
	if !strings.HasPrefix(normalizedInput, "/") {
		return PathDescriptor{}, faults.NewTypedError(
			faults.ValidationError,
			"logical path must be absolute",
			nil,
		)
	}
	trailingCollectionMarker := strings.HasSuffix(normalizedInput, "/")

	rawSegments := strings.Split(normalizedInput, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, segment := range rawSegments {
		if segment == "" || segment == "." {
			continue
		}
		if segment == ".." {
			return PathDescriptor{}, faults.NewTypedError(
				faults.ValidationError,
				"logical path must not contain traversal segments",
				nil,
			)
		}
		if hasWildcardPattern(segment) {
			if _, err := path.Match(segment, "sample"); err != nil {
				return PathDescriptor{}, faults.NewTypedError(
					faults.ValidationError,
					"logical path contains invalid wildcard expression",
					err,
				)
			}
		}
		segments = append(segments, segment)
	}

	collectionTarget := trailingCollectionMarker
	selectorMode := trailingCollectionMarker
	if len(segments) > 0 && segments[len(segments)-1] == "_" {
		collectionTarget = true
		selectorMode = true
		segments = segments[:len(segments)-1]
	}
	for _, segment := range segments {
		if segment == "_" || hasWildcardPattern(segment) {
			collectionTarget = true
			selectorMode = true
		}
	}

	selector := "/"
	if len(segments) > 0 {
		selector = "/" + strings.Join(segments, "/")
	}
	selector = path.Clean(selector)
	if !strings.HasPrefix(selector, "/") {
		return PathDescriptor{}, faults.NewTypedError(
			faults.ValidationError,
			"logical path must be absolute",
			nil,
		)
	}
	if selector != "/" {
		selector = strings.TrimSuffix(selector, "/")
	}

	return PathDescriptor{
		Selector:     selector,
		Segments:     splitPathSegments(selector),
		Collection:   collectionTarget,
		SelectorMode: selectorMode,
	}, nil
}
