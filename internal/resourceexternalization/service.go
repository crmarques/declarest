package resourceexternalization

import (
	"context"
	"fmt"
	pathpkg "path"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

type ArtifactReader interface {
	ReadResourceArtifact(ctx context.Context, logicalPath string, file string) ([]byte, error)
}

type ExtractResult struct {
	Payload   resource.Value
	Artifacts []repository.ResourceArtifact
}

type externalizedTarget struct {
	ConcretePath []string
	File         string
	Placeholder  string
	Value        any
}

func Extract(value resource.Value, entries []metadata.ResolvedExternalizedAttribute) (ExtractResult, error) {
	normalized, err := resource.Normalize(value)
	if err != nil {
		return ExtractResult{}, err
	}
	if len(entries) == 0 {
		return ExtractResult{Payload: normalized}, nil
	}

	cloned, err := cloneValue(normalized)
	if err != nil {
		return ExtractResult{}, err
	}

	targets, err := resolveExternalizedTargets(cloned, entries)
	if err != nil {
		return ExtractResult{}, err
	}

	artifacts := make([]repository.ResourceArtifact, 0, len(targets))
	for _, target := range targets {
		textValue, ok := target.Value.(string)
		if !ok {
			return ExtractResult{}, faults.NewValidationError(
				fmt.Sprintf("externalized attribute %s must be a string value", formatAttributePath(target.ConcretePath)),
				nil,
			)
		}

		if err := assignPathValue(cloned, target.ConcretePath, target.Placeholder); err != nil {
			return ExtractResult{}, wrapPathError(target.ConcretePath, "replace externalized attribute placeholder", err)
		}
		artifacts = append(artifacts, repository.ResourceArtifact{
			File:    target.File,
			Content: []byte(textValue),
		})
	}

	result, err := resource.Normalize(cloned)
	if err != nil {
		return ExtractResult{}, err
	}

	return ExtractResult{
		Payload:   result,
		Artifacts: artifacts,
	}, nil
}

func Expand(
	ctx context.Context,
	reader ArtifactReader,
	logicalPath string,
	value resource.Value,
	entries []metadata.ResolvedExternalizedAttribute,
) (resource.Value, error) {
	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return normalized, nil
	}

	cloned, err := cloneValue(normalized)
	if err != nil {
		return nil, err
	}

	targets, err := resolveExternalizedTargets(cloned, entries)
	if err != nil {
		return nil, err
	}

	for _, target := range targets {
		textValue, ok := target.Value.(string)
		if !ok {
			continue
		}

		if textValue != target.Placeholder {
			continue
		}
		if reader == nil {
			return nil, faults.NewValidationError(
				fmt.Sprintf(
					"externalized attribute %s requires a configured repository artifact reader",
					formatAttributePath(target.ConcretePath),
				),
				nil,
			)
		}

		content, err := reader.ReadResourceArtifact(ctx, logicalPath, target.File)
		if err != nil {
			if faults.IsCategory(err, faults.NotFoundError) {
				return nil, faults.NewValidationError(
					fmt.Sprintf(
						"externalized attribute %s references missing file %q",
						formatAttributePath(target.ConcretePath),
						target.File,
					),
					err,
				)
			}
			return nil, err
		}

		if err := assignPathValue(cloned, target.ConcretePath, string(content)); err != nil {
			return nil, wrapPathError(target.ConcretePath, "expand externalized attribute", err)
		}
	}

	return resource.Normalize(cloned)
}

func cloneValue(value any) (any, error) {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			clonedChild, err := cloneValue(child)
			if err != nil {
				return nil, err
			}
			result[key] = clonedChild
		}
		return result, nil
	case []any:
		result := make([]any, len(typed))
		for idx := range typed {
			clonedChild, err := cloneValue(typed[idx])
			if err != nil {
				return nil, err
			}
			result[idx] = clonedChild
		}
		return result, nil
	default:
		return typed, nil
	}
}

func resolveExternalizedTargets(
	value any,
	entries []metadata.ResolvedExternalizedAttribute,
) ([]externalizedTarget, error) {
	targets := make([]externalizedTarget, 0, len(entries))
	seenConcretePaths := map[string]struct{}{}
	seenFiles := map[string]struct{}{}

	for _, entry := range entries {
		matches, err := resolvePathMatches(value, entry.Path)
		if err != nil {
			return nil, wrapPathError(entry.Path, "resolve externalized attribute", err)
		}

		for _, match := range matches {
			concreteKey := strings.Join(match.ConcretePath, "\x00")
			if _, exists := seenConcretePaths[concreteKey]; exists {
				return nil, faults.NewValidationError(
					fmt.Sprintf(
						"externalized attribute %s resolves duplicate concrete path %s",
						formatAttributePath(entry.Path),
						formatAttributePath(match.ConcretePath),
					),
					nil,
				)
			}
			seenConcretePaths[concreteKey] = struct{}{}

			file := resolveArtifactFile(entry.File, match.WildcardIndices)
			if _, exists := seenFiles[file]; exists {
				return nil, faults.NewValidationError(
					fmt.Sprintf(
						"externalized attribute %s resolves duplicate artifact file %q",
						formatAttributePath(entry.Path),
						file,
					),
					nil,
				)
			}
			seenFiles[file] = struct{}{}

			targets = append(targets, externalizedTarget{
				ConcretePath: match.ConcretePath,
				File:         file,
				Placeholder:  fmt.Sprintf(entry.Template, file),
				Value:        match.Value,
			})
		}
	}

	return targets, nil
}

type pathMatch struct {
	ConcretePath    []string
	WildcardIndices []int
	Value           any
}

func resolvePathMatches(value any, path []string) ([]pathMatch, error) {
	if len(path) == 0 {
		return nil, nil
	}

	return collectPathMatches(value, path, nil, nil)
}

func collectPathMatches(
	current any,
	path []string,
	concretePath []string,
	wildcardIndices []int,
) ([]pathMatch, error) {
	if len(path) == 0 {
		return []pathMatch{{
			ConcretePath:    append([]string(nil), concretePath...),
			WildcardIndices: append([]int(nil), wildcardIndices...),
			Value:           current,
		}}, nil
	}

	segment := path[0]
	switch typed := current.(type) {
	case map[string]any:
		child, found := typed[segment]
		if !found {
			return nil, nil
		}
		return collectPathMatches(child, path[1:], appendPathSegment(concretePath, segment), wildcardIndices)
	case []any:
		if segment == "*" {
			matches := make([]pathMatch, 0, len(typed))
			for idx := range typed {
				childMatches, err := collectPathMatches(
					typed[idx],
					path[1:],
					appendPathSegment(concretePath, strconv.Itoa(idx)),
					appendWildcardIndex(wildcardIndices, idx),
				)
				if err != nil {
					return nil, err
				}
				matches = append(matches, childMatches...)
			}
			return matches, nil
		}

		index, ok := parseArrayIndex(segment)
		if !ok {
			return nil, faults.NewValidationError(
				fmt.Sprintf(
					"externalized attribute path %s must use \"*\" or a numeric index before segment %q",
					formatAttributePath(concretePath),
					segment,
				),
				nil,
			)
		}
		if index < 0 || index >= len(typed) {
			return nil, nil
		}
		return collectPathMatches(typed[index], path[1:], appendPathSegment(concretePath, strconv.Itoa(index)), wildcardIndices)
	default:
		return nil, faults.NewValidationError(
			fmt.Sprintf(
				"externalized attribute path %s crosses a non-traversable value before segment %q",
				formatAttributePath(concretePath),
				segment,
			),
			nil,
		)
	}
}

func assignPathValue(value any, path []string, replacement any) error {
	if len(path) == 0 {
		return faults.NewValidationError("externalized attribute path must not be empty", nil)
	}

	current := value
	for idx := 0; idx < len(path)-1; idx++ {
		segment := path[idx]

		switch typed := current.(type) {
		case map[string]any:
			next, found := typed[segment]
			if !found {
				return faults.NewValidationError(
					fmt.Sprintf("externalized attribute path %s is missing", formatAttributePath(path[:idx+1])),
					nil,
				)
			}
			current = next
		case []any:
			index, ok := parseArrayIndex(segment)
			if !ok || index < 0 || index >= len(typed) {
				return faults.NewValidationError(
					fmt.Sprintf("externalized attribute path %s is missing", formatAttributePath(path[:idx+1])),
					nil,
				)
			}
			current = typed[index]
		default:
			return faults.NewValidationError(
				fmt.Sprintf("externalized attribute path %s crosses a non-object value", formatAttributePath(path[:idx+1])),
				nil,
			)
		}
	}

	lastSegment := path[len(path)-1]
	switch typed := current.(type) {
	case map[string]any:
		typed[lastSegment] = replacement
		return nil
	case []any:
		index, ok := parseArrayIndex(lastSegment)
		if !ok || index < 0 || index >= len(typed) {
			return faults.NewValidationError(
				fmt.Sprintf("externalized attribute path %s is missing", formatAttributePath(path)),
				nil,
			)
		}
		typed[index] = replacement
		return nil
	default:
		return faults.NewValidationError(
			fmt.Sprintf("externalized attribute path %s crosses a non-object value", formatAttributePath(path[:len(path)-1])),
			nil,
		)
	}
}

func resolveArtifactFile(base string, wildcardIndices []int) string {
	if len(wildcardIndices) == 0 {
		return base
	}

	dir := pathpkg.Dir(base)
	name := pathpkg.Base(base)
	ext := pathpkg.Ext(name)
	stem := strings.TrimSuffix(name, ext)
	if stem == "" {
		stem = name
		ext = ""
	}

	var builder strings.Builder
	builder.WriteString(stem)
	for _, idx := range wildcardIndices {
		builder.WriteString("-")
		builder.WriteString(strconv.Itoa(idx))
	}
	builder.WriteString(ext)

	if dir == "." {
		return builder.String()
	}
	return pathpkg.Join(dir, builder.String())
}

func appendPathSegment(path []string, segment string) []string {
	out := append([]string(nil), path...)
	out = append(out, segment)
	return out
}

func appendWildcardIndex(indices []int, index int) []int {
	out := append([]int(nil), indices...)
	out = append(out, index)
	return out
}

func parseArrayIndex(value string) (int, bool) {
	if value == "" {
		return 0, false
	}
	index, err := strconv.Atoi(value)
	if err != nil || index < 0 {
		return 0, false
	}
	return index, true
}

func formatAttributePath(path []string) string {
	if len(path) == 0 {
		return "[]"
	}

	formatted := "["
	for idx, segment := range path {
		if idx > 0 {
			formatted += ", "
		}
		formatted += fmt.Sprintf("%q", segment)
	}
	formatted += "]"
	return formatted
}

func wrapPathError(path []string, action string, err error) error {
	if err == nil {
		return nil
	}
	return faults.NewValidationError(
		fmt.Sprintf("%s at externalized attribute %s", action, formatAttributePath(path)),
		err,
	)
}
