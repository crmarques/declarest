package resourceexternalization

import (
	"context"
	"fmt"

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

	artifacts := make([]repository.ResourceArtifact, 0, len(entries))
	for _, entry := range entries {
		currentValue, found, err := lookupPathValue(cloned, entry.Path)
		if err != nil {
			return ExtractResult{}, wrapPathError(entry.Path, "resolve externalized attribute", err)
		}
		if !found {
			continue
		}

		textValue, ok := currentValue.(string)
		if !ok {
			return ExtractResult{}, faults.NewValidationError(
				fmt.Sprintf("externalized attribute %s must be a string value", formatAttributePath(entry.Path)),
				nil,
			)
		}

		placeholder := fmt.Sprintf(entry.Template, entry.File)
		if err := assignPathValue(cloned, entry.Path, placeholder, false); err != nil {
			return ExtractResult{}, wrapPathError(entry.Path, "replace externalized attribute placeholder", err)
		}
		artifacts = append(artifacts, repository.ResourceArtifact{
			File:    entry.File,
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

	for _, entry := range entries {
		currentValue, found, err := lookupPathValue(cloned, entry.Path)
		if err != nil {
			return nil, wrapPathError(entry.Path, "resolve externalized attribute", err)
		}
		if !found {
			continue
		}

		textValue, ok := currentValue.(string)
		if !ok {
			continue
		}

		placeholder := fmt.Sprintf(entry.Template, entry.File)
		if textValue != placeholder {
			continue
		}
		if reader == nil {
			return nil, faults.NewValidationError(
				fmt.Sprintf(
					"externalized attribute %s requires a configured repository artifact reader",
					formatAttributePath(entry.Path),
				),
				nil,
			)
		}

		content, err := reader.ReadResourceArtifact(ctx, logicalPath, entry.File)
		if err != nil {
			if faults.IsCategory(err, faults.NotFoundError) {
				return nil, faults.NewValidationError(
					fmt.Sprintf(
						"externalized attribute %s references missing file %q",
						formatAttributePath(entry.Path),
						entry.File,
					),
					err,
				)
			}
			return nil, err
		}

		if err := assignPathValue(cloned, entry.Path, string(content), false); err != nil {
			return nil, wrapPathError(entry.Path, "expand externalized attribute", err)
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

func lookupPathValue(value any, path []string) (any, bool, error) {
	current := value
	for idx, segment := range path {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false, faults.NewValidationError(
				fmt.Sprintf(
					"externalized attribute path %s crosses non-object segment %q",
					formatAttributePath(path[:idx]),
					segment,
				),
				nil,
			)
		}

		child, found := object[segment]
		if !found {
			return nil, false, nil
		}
		current = child
	}

	return current, true, nil
}

func assignPathValue(value any, path []string, replacement any, createMissing bool) error {
	current, ok := value.(map[string]any)
	if !ok {
		return faults.NewValidationError("externalized attribute root payload must be an object", nil)
	}

	for idx := 0; idx < len(path)-1; idx++ {
		segment := path[idx]
		next, found := current[segment]
		if !found {
			if !createMissing {
				return faults.NewValidationError(
					fmt.Sprintf("externalized attribute path %s is missing", formatAttributePath(path[:idx+1])),
					nil,
				)
			}
			child := map[string]any{}
			current[segment] = child
			current = child
			continue
		}

		child, ok := next.(map[string]any)
		if !ok {
			return faults.NewValidationError(
				fmt.Sprintf("externalized attribute path %s crosses a non-object value", formatAttributePath(path[:idx+1])),
				nil,
			)
		}
		current = child
	}

	current[path[len(path)-1]] = replacement
	return nil
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
