// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

type selectorMatch struct {
	selector          string
	matchedCollection string
}

type descendantRuntimeContext struct {
	matchedCollectionPath string
}

type resolvedMetadataResult struct {
	metadata   metadatadomain.ResourceMetadata
	descendant *descendantRuntimeContext
}

func (s *FSMetadataService) ResolveForPath(ctx context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	result, err := s.resolveForPathWithContext(ctx, logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	return result.metadata, nil
}

func (s *FSMetadataService) resolveForPathWithContext(
	ctx context.Context,
	logicalPath string,
) (resolvedMetadataResult, error) {
	debugctx.Printf(ctx, "metadata fs resolve start logical_path=%q base_dir=%q", logicalPath, s.baseDir)

	target, err := normalizeResolvePath(logicalPath)
	if err != nil {
		debugctx.Printf(ctx, "metadata fs resolve invalid logical_path=%q error=%v", logicalPath, err)
		return resolvedMetadataResult{}, err
	}
	debugctx.Printf(
		ctx,
		"metadata fs resolve normalized logical_path=%q normalized=%q collection=%t",
		logicalPath,
		target.path,
		target.collection,
	)

	result := resolvedMetadataResult{metadata: metadatadomain.ResourceMetadata{}}

	apply := func(match selectorMatch, kind metadataPathKind) error {
		targetMetadataPath, pathErr := s.metadataFilePath(match.selector, kind)
		if pathErr != nil {
			debugctx.Printf(
				ctx,
				"metadata fs resolve resolve-path failed selector=%q kind=%q error=%v",
				match.selector,
				metadataPathKindName(kind),
				pathErr,
			)
			return pathErr
		}
		debugctx.Printf(
			ctx,
			"metadata fs resolve lookup selector=%q kind=%q file=%q",
			match.selector,
			metadataPathKindName(kind),
			targetMetadataPath,
		)

		item, found, err := s.tryReadMetadata(match.selector, kind)
		if err != nil {
			debugctx.Printf(
				ctx,
				"metadata fs resolve failed selector=%q kind=%q file=%q error=%v",
				match.selector,
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
				match.selector,
				metadataPathKindName(kind),
				targetMetadataPath,
			)
			return nil
		}
		resolvedItem, resolveErr := s.resolveMetadataDefaults(ctx, match.selector, kind, item)
		if resolveErr != nil {
			return resolveErr
		}

		if kind == metadataPathCollection && !collectionMetadataAppliesToTarget(target, match.matchedCollection, resolvedItem.Selector) {
			debugctx.Printf(
				ctx,
				"metadata fs resolve skip selector=%q kind=%q matched_collection=%q target=%q",
				match.selector,
				metadataPathKindName(kind),
				match.matchedCollection,
				target.path,
			)
			return nil
		}

		result.metadata = metadatadomain.MergeResourceMetadata(result.metadata, resolvedItem)
		if kind == metadataPathCollection && resolvedItem.Selector.AllowsDescendants() {
			result.descendant = preferDescendantContext(
				result.descendant,
				&descendantRuntimeContext{matchedCollectionPath: match.matchedCollection},
			)
		}
		debugctx.Printf(
			ctx,
			"metadata fs resolve hit selector=%q kind=%q file=%q matched_collection=%q",
			match.selector,
			metadataPathKindName(kind),
			targetMetadataPath,
			match.matchedCollection,
		)
		return nil
	}

	if err := apply(selectorMatch{selector: "/", matchedCollection: "/"}, metadataPathCollection); err != nil {
		return resolvedMetadataResult{}, err
	}

	segments := splitPathSegments(target.path)
	parentMatches := []selectorMatch{{selector: "/", matchedCollection: "/"}}
	for _, segment := range segments {
		wildcardCandidates := map[string]selectorMatch{}
		literalCandidates := map[string]selectorMatch{}
		nextParents := map[string]selectorMatch{}

		for _, parentMatch := range parentMatches {
			wildcards, literals, err := s.matchingCollectionCandidates(parentMatch.selector, segment)
			if err != nil {
				debugctx.Printf(
					ctx,
					"metadata fs resolve match failed parent=%q segment=%q error=%v",
					parentMatch.selector,
					segment,
					err,
				)
				return resolvedMetadataResult{}, err
			}

			for _, selector := range wildcards {
				match := selectorMatch{
					selector:          selector,
					matchedCollection: joinConcreteCollectionPath(parentMatch.matchedCollection, segment),
				}
				wildcardCandidates[selectorMatchKey(match)] = match
				nextParents[selectorMatchKey(match)] = match
			}
			for _, selector := range literals {
				match := selectorMatch{
					selector:          selector,
					matchedCollection: joinConcreteCollectionPath(parentMatch.matchedCollection, segment),
				}
				literalCandidates[selectorMatchKey(match)] = match
				nextParents[selectorMatchKey(match)] = match
			}
		}

		for _, match := range sortedSelectorMatches(wildcardCandidates) {
			if err := apply(match, metadataPathCollection); err != nil {
				return resolvedMetadataResult{}, err
			}
		}
		for _, match := range sortedSelectorMatches(literalCandidates) {
			if err := apply(match, metadataPathCollection); err != nil {
				return resolvedMetadataResult{}, err
			}
		}

		parentMatches = sortedSelectorMatches(nextParents)
		if len(parentMatches) == 0 {
			break
		}
	}

	if !target.collection && target.path != "/" {
		if err := apply(selectorMatch{selector: target.path, matchedCollection: collectionPathForLogicalPath(target.path)}, metadataPathResource); err != nil {
			return resolvedMetadataResult{}, err
		}
	}

	if _, err := metadatadomain.ResolveEffectiveDefaults(result.metadata.Defaults); err != nil {
		return resolvedMetadataResult{}, err
	}

	debugctx.Printf(ctx, "metadata fs resolve done logical_path=%q normalized=%q", logicalPath, target.path)
	return result, nil
}

func collectionMetadataAppliesToTarget(
	target resolvedPathTarget,
	matchedCollection string,
	selector *metadatadomain.SelectorSpec,
) bool {
	if matchedCollection == "/" {
		return true
	}
	if target.path == matchedCollection {
		return true
	}

	targetCollection := target.path
	if !target.collection {
		targetCollection = collectionPathForLogicalPath(target.path)
	}
	if targetCollection == matchedCollection {
		return true
	}
	if !selector.AllowsDescendants() {
		return false
	}
	return isDescendantCollectionPath(matchedCollection, targetCollection)
}

func isDescendantCollectionPath(ancestor string, candidate string) bool {
	if ancestor == "/" {
		return candidate != "/"
	}
	return strings.HasPrefix(candidate, ancestor+"/")
}

func preferDescendantContext(
	current *descendantRuntimeContext,
	next *descendantRuntimeContext,
) *descendantRuntimeContext {
	if next == nil {
		return current
	}
	if current == nil {
		return next
	}
	if selectorDepth(next.matchedCollectionPath) >= selectorDepth(current.matchedCollectionPath) {
		return next
	}
	return current
}

func selectorDepth(value string) int {
	return len(splitPathSegments(value))
}

func joinConcreteCollectionPath(parent string, segment string) string {
	if parent == "/" {
		return "/" + segment
	}
	return parent + "/" + segment
}

func selectorMatchKey(value selectorMatch) string {
	return value.selector + "\x00" + value.matchedCollection
}

func sortedSelectorMatches(values map[string]selectorMatch) []selectorMatch {
	if len(values) == 0 {
		return nil
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	result := make([]selectorMatch, 0, len(keys))
	for _, key := range keys {
		result = append(result, values[key])
	}
	return result
}

// walkChildSelectors traverses child directory entries under the metadata
// selectors matching logicalPath. The visitor receives each directory child
// name and its parent selector. Return true from the visitor to stop early.
func (s *FSMetadataService) walkChildSelectors(
	logicalPath string,
	visitor func(childName string, parentSelector string) bool,
) error {
	target, err := normalizeResolvePath(logicalPath)
	if err != nil {
		return err
	}

	parentSelectors, err := s.matchingParentSelectors(target.path)
	if err != nil {
		return err
	}

	for _, parentSelector := range parentSelectors {
		parentDir, dirErr := s.selectorDirPath(parentSelector)
		if dirErr != nil {
			return dirErr
		}

		entries, readErr := os.ReadDir(parentDir)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			return faults.Internal("failed to list metadata selector children", readErr)
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			childName := strings.TrimSpace(entry.Name())
			if childName == "" {
				continue
			}
			if visitor(childName, parentSelector) {
				return nil
			}
		}
	}
	return nil
}

// ResolveCollectionChildren returns literal child collection selector segments
// available for the given logical path based on metadata selector structure.
// It is used by shell completion to surface metadata-only branches (for example
// intermediary "/_/" templates) even when OpenAPI paths differ.
func (s *FSMetadataService) ResolveCollectionChildren(ctx context.Context, logicalPath string) ([]string, error) {
	debugctx.Printf(ctx, "metadata fs resolve-children start logical_path=%q base_dir=%q", logicalPath, s.baseDir)

	children := map[string]struct{}{}
	err := s.walkChildSelectors(logicalPath, func(childName string, _ string) bool {
		if childName != "_" && !hasWildcardPattern(childName) {
			children[childName] = struct{}{}
		}
		return false
	})
	if err != nil {
		debugctx.Printf(ctx, "metadata fs resolve-children failed logical_path=%q error=%v", logicalPath, err)
		return nil, err
	}

	resolved := sortedSelectorKeys(children)
	debugctx.Printf(ctx, "metadata fs resolve-children done logical_path=%q children=%v", logicalPath, resolved)
	return resolved, nil
}

// HasCollectionWildcardChild reports true when any metadata child selector
// under the provided logical path uses a wildcard segment (for example "_").
// It is used by fallback helpers that need to know when metadata allows access
// to selectors that are not literal directory names.
func (s *FSMetadataService) HasCollectionWildcardChild(ctx context.Context, logicalPath string) (bool, error) {
	debugctx.Printf(ctx, "metadata fs wildcard-child check start logical_path=%q base_dir=%q", logicalPath, s.baseDir)

	found := false
	err := s.walkChildSelectors(logicalPath, func(childName string, parentSelector string) bool {
		if childName == "_" || hasWildcardPattern(childName) {
			debugctx.Printf(
				ctx,
				"metadata fs wildcard-child match logical_path=%q selector=%q child=%q",
				logicalPath,
				parentSelector,
				childName,
			)
			found = true
			return true
		}
		return false
	})
	if err != nil {
		debugctx.Printf(ctx, "metadata fs wildcard-child failed logical_path=%q error=%v", logicalPath, err)
		return false, err
	}
	return found, nil
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
		return nil, nil, faults.Internal("failed to list metadata selectors", err)
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
				return nil, nil, faults.Invalid(
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

type resolvedPathTarget struct {
	path       string
	collection bool
}

func normalizeResolvePath(logicalPath string) (resolvedPathTarget, error) {
	descriptor, err := metadatadomain.ParsePathDescriptor(logicalPath)
	if err != nil {
		return resolvedPathTarget{}, err
	}

	return resolvedPathTarget{
		path:       descriptor.Selector,
		collection: descriptor.Collection,
	}, nil
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
