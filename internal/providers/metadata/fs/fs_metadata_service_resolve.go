package fsmetadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (s *FSMetadataService) ResolveForPath(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	targetPath, err := normalizeResolvePath(logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	merged := metadatadomain.ResourceMetadata{}

	apply := func(selector string, kind metadataPathKind) error {
		item, found, err := s.tryReadMetadata(selector, kind)
		if err != nil {
			return err
		}
		if !found {
			return nil
		}
		merged = metadatadomain.MergeResourceMetadata(merged, item)
		return nil
	}

	if err := apply("/", metadataPathCollection); err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	segments := splitPathSegments(targetPath)
	parentSelector := "/"
	for _, segment := range segments {
		wildcards, literals, err := s.matchingCollectionCandidates(parentSelector, segment)
		if err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}

		for _, selector := range wildcards {
			if err := apply(selector, metadataPathCollection); err != nil {
				return metadatadomain.ResourceMetadata{}, err
			}
		}
		for _, selector := range literals {
			if err := apply(selector, metadataPathCollection); err != nil {
				return metadatadomain.ResourceMetadata{}, err
			}
		}

		parentSelector = joinSelector(parentSelector, segment)
	}

	if targetPath != "/" {
		if err := apply(targetPath, metadataPathResource); err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}
	}

	return merged, nil
}

func (s *FSMetadataService) matchingCollectionCandidates(parentSelector string, segment string) ([]string, []string, error) {
	parentDir, err := s.selectorDirPath(parentSelector)
	if err != nil {
		return nil, nil, err
	}

	entries, err := os.ReadDir(parentDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil, nil
		}
		return nil, nil, internalError("failed to list metadata selectors", err)
	}

	wildcards := make([]string, 0)
	literals := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "_" {
			continue
		}

		childName := entry.Name()
		childSelector := joinSelector(parentSelector, childName)

		collectionMetadataPath, pathErr := s.metadataFilePath(childSelector, metadataPathCollection)
		if pathErr != nil {
			return nil, nil, pathErr
		}
		if _, statErr := os.Stat(collectionMetadataPath); statErr != nil {
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return nil, nil, internalError("failed to inspect metadata selector file", statErr)
		}

		if hasWildcardPattern(childName) {
			matched, matchErr := path.Match(childName, segment)
			if matchErr != nil {
				return nil, nil, validationError(
					fmt.Sprintf("invalid wildcard selector %q", childSelector),
					matchErr,
				)
			}
			if matched {
				wildcards = append(wildcards, childSelector)
			}
			continue
		}

		if childName == segment {
			literals = append(literals, childSelector)
		}
	}

	sort.Strings(wildcards)
	sort.Strings(literals)
	return wildcards, literals, nil
}

func normalizeResolvePath(logicalPath string) (string, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return "", err
	}

	for _, segment := range splitPathSegments(normalizedPath) {
		if hasWildcardPattern(segment) {
			return "", validationError("resolve path must not contain wildcard segments", nil)
		}
	}

	return normalizedPath, nil
}

func splitPathSegments(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func joinSelector(parent string, child string) string {
	if parent == "/" {
		return "/" + child
	}
	return parent + "/" + child
}

func hasWildcardPattern(segment string) bool {
	return strings.ContainsAny(segment, "*?[")
}

func aliasForLogicalPath(logicalPath string) string {
	if logicalPath == "/" {
		return "/"
	}
	return path.Base(logicalPath)
}

func collectionPathForLogicalPath(logicalPath string) string {
	if logicalPath == "/" {
		return "/"
	}
	collectionPath := path.Dir(logicalPath)
	if collectionPath == "." || collectionPath == "" {
		return "/"
	}
	return collectionPath
}

func sortedOperationKeys(values map[string]metadatadomain.OperationSpec) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
