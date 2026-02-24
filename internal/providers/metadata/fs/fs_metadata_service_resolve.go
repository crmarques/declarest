package fsmetadata

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (s *FSMetadataService) ResolveForPath(ctx context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	debugctx.Printf(ctx, "metadata fs resolve start logical_path=%q base_dir=%q", logicalPath, s.baseDir)

	targetPath, err := normalizeResolvePath(logicalPath)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs resolve invalid logical_path=%q error=%v", logicalPath, err)
		return metadatadomain.ResourceMetadata{}, err
	}
	debugctx.Printf(ctx, "metadata fs resolve normalized logical_path=%q normalized=%q", logicalPath, targetPath)

	merged := metadatadomain.ResourceMetadata{}

	apply := func(selector string, kind metadataPathKind) error {
		targetMetadataPath, pathErr := s.metadataFilePath(selector, kind)
		if pathErr != nil {
			debugctx.Printf(
				ctx,
				"metadata fs resolve resolve-path failed selector=%q kind=%q error=%v",
				selector,
				metadataPathKindName(kind),
				pathErr,
			)
			return pathErr
		}
		debugctx.Printf(
			ctx,
			"metadata fs resolve lookup selector=%q kind=%q file=%q",
			selector,
			metadataPathKindName(kind),
			targetMetadataPath,
		)

		item, found, err := s.tryReadMetadata(selector, kind)
		if err != nil {
			debugctx.Printf(
				ctx,
				"metadata fs resolve failed selector=%q kind=%q file=%q error=%v",
				selector,
				metadataPathKindName(kind),
				targetMetadataPath,
				err,
			)
			return err
		}
		if !found {
			debugctx.Printf(
				ctx,
				"metadata fs resolve miss selector=%q kind=%q file=%q",
				selector,
				metadataPathKindName(kind),
				targetMetadataPath,
			)
			return nil
		}
		merged = metadatadomain.MergeResourceMetadata(merged, item)
		debugctx.Printf(
			ctx,
			"metadata fs resolve hit selector=%q kind=%q file=%q",
			selector,
			metadataPathKindName(kind),
			targetMetadataPath,
		)
		return nil
	}

	if err := apply("/", metadataPathCollection); err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}

	segments := splitPathSegments(targetPath)
	parentSelectors := []string{"/"}
	for _, segment := range segments {
		wildcardCandidates := make(map[string]struct{})
		literalCandidates := make(map[string]struct{})
		nextParents := make(map[string]struct{})

		for _, parentSelector := range parentSelectors {
			wildcards, literals, err := s.matchingCollectionCandidates(parentSelector, segment)
			if err != nil {
				debugctx.Printf(
					ctx,
					"metadata fs resolve match failed parent=%q segment=%q error=%v",
					parentSelector,
					segment,
					err,
				)
				return metadatadomain.ResourceMetadata{}, err
			}

			for _, selector := range wildcards {
				wildcardCandidates[selector] = struct{}{}
				nextParents[selector] = struct{}{}
			}
			for _, selector := range literals {
				literalCandidates[selector] = struct{}{}
				nextParents[selector] = struct{}{}
			}
		}

		for _, selector := range sortedSelectorKeys(wildcardCandidates) {
			if err := apply(selector, metadataPathCollection); err != nil {
				return metadatadomain.ResourceMetadata{}, err
			}
		}
		for _, selector := range sortedSelectorKeys(literalCandidates) {
			if err := apply(selector, metadataPathCollection); err != nil {
				return metadatadomain.ResourceMetadata{}, err
			}
		}

		parentSelectors = sortedSelectorKeys(nextParents)
		if len(parentSelectors) == 0 {
			break
		}
	}

	if targetPath != "/" {
		if err := apply(targetPath, metadataPathResource); err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}
	}

	debugctx.Printf(ctx, "metadata fs resolve done logical_path=%q normalized=%q", logicalPath, targetPath)
	return merged, nil
}

// ResolveCollectionChildren returns literal child collection selector segments
// available for the given logical path based on metadata selector structure.
// It is used by shell completion to surface metadata-only branches (for example
// intermediary "/_/" templates) even when OpenAPI paths differ.
func (s *FSMetadataService) ResolveCollectionChildren(ctx context.Context, logicalPath string) ([]string, error) {
	debugctx.Printf(ctx, "metadata fs resolve-children start logical_path=%q base_dir=%q", logicalPath, s.baseDir)

	targetPath, err := normalizeResolvePath(logicalPath)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs resolve-children invalid logical_path=%q error=%v", logicalPath, err)
		return nil, err
	}

	parentSelectors, err := s.matchingParentSelectors(targetPath)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs resolve-children match failed logical_path=%q error=%v", targetPath, err)
		return nil, err
	}
	if len(parentSelectors) == 0 {
		return nil, nil
	}

	children := map[string]struct{}{}
	for _, parentSelector := range parentSelectors {
		parentDir, dirErr := s.selectorDirPath(parentSelector)
		if dirErr != nil {
			return nil, dirErr
		}

		entries, readErr := os.ReadDir(parentDir)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			return nil, internalError("failed to list metadata selector children", readErr)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			childName := strings.TrimSpace(entry.Name())
			if childName == "" || childName == "_" || hasWildcardPattern(childName) {
				continue
			}
			children[childName] = struct{}{}
		}
	}

	resolved := sortedSelectorKeys(children)
	debugctx.Printf(
		ctx,
		"metadata fs resolve-children done logical_path=%q normalized=%q children=%v",
		logicalPath,
		targetPath,
		resolved,
	)
	return resolved, nil
}

// HasCollectionWildcardChild reports true when any metadata child selector
// under the provided logical path uses a wildcard segment (for example "_").
// It is used by fallback helpers that need to know when metadata allows access
// to selectors that are not literal directory names.
func (s *FSMetadataService) HasCollectionWildcardChild(ctx context.Context, logicalPath string) (bool, error) {
	debugctx.Printf(ctx, "metadata fs wildcard-child check start logical_path=%q base_dir=%q", logicalPath, s.baseDir)

	targetPath, err := normalizeResolvePath(logicalPath)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs wildcard-child invalid logical_path=%q error=%v", logicalPath, err)
		return false, err
	}

	parentSelectors, err := s.matchingParentSelectors(targetPath)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs wildcard-child match failed logical_path=%q error=%v", targetPath, err)
		return false, err
	}
	if len(parentSelectors) == 0 {
		return false, nil
	}

	for _, parentSelector := range parentSelectors {
		parentDir, dirErr := s.selectorDirPath(parentSelector)
		if dirErr != nil {
			return false, dirErr
		}

		entries, readErr := os.ReadDir(parentDir)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			return false, internalError("failed to list metadata selector children", readErr)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			childName := strings.TrimSpace(entry.Name())
			if childName == "_" || hasWildcardPattern(childName) {
				debugctx.Printf(
					ctx,
					"metadata fs wildcard-child match logical_path=%q selector=%q child=%q",
					logicalPath,
					parentSelector,
					childName,
				)
				return true, nil
			}
		}
	}

	return false, nil
}

func (s *FSMetadataService) matchingParentSelectors(logicalPath string) ([]string, error) {
	parentSelectors := []string{"/"}
	for _, segment := range splitPathSegments(logicalPath) {
		nextParents := map[string]struct{}{}
		for _, parentSelector := range parentSelectors {
			wildcards, literals, err := s.matchingCollectionCandidates(parentSelector, segment)
			if err != nil {
				return nil, err
			}
			for _, selector := range wildcards {
				nextParents[selector] = struct{}{}
			}
			for _, selector := range literals {
				nextParents[selector] = struct{}{}
			}
		}

		parentSelectors = sortedSelectorKeys(nextParents)
		if len(parentSelectors) == 0 {
			return nil, nil
		}
	}
	return parentSelectors, nil
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
		if !entry.IsDir() {
			continue
		}

		childName := entry.Name()
		childSelector := joinSelector(parentSelector, childName)

		// "_" is used in repository templates as an intermediary wildcard selector.
		if childName == "_" {
			wildcards = append(wildcards, childSelector)
			continue
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

func sortedSelectorKeys(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
