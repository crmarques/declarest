package metadata

import (
	"fmt"
	"path"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

const (
	DefaultExternalizedAttributeTemplate = "{{include %s}}"

	ExternalizedAttributeModeText                = "text"
	ExternalizedAttributeSaveBehaviorExternalize = "externalize"
	ExternalizedAttributeRenderBehaviorInclude   = "include"
)

func ResolveExternalizedAttributes(metadata ResourceMetadata) ([]ResolvedExternalizedAttribute, error) {
	if metadata.ExternalizedAttributes == nil {
		return nil, nil
	}

	resolved := make([]ResolvedExternalizedAttribute, 0, len(metadata.ExternalizedAttributes))
	pathIndexByKey := map[string]int{}
	fileIndexByKey := map[string]int{}

	for idx, item := range metadata.ExternalizedAttributes {
		if item.Enabled != nil && !*item.Enabled {
			continue
		}

		entry, err := resolveExternalizedAttribute(item, idx)
		if err != nil {
			return nil, err
		}

		if previous, exists := pathIndexByKey[entry.Path]; exists {
			return nil, faults.NewValidationError(
				fmt.Sprintf(
					"resourceInfo.externalizedAttributes[%d].path duplicates resourceInfo.externalizedAttributes[%d].path",
					idx,
					previous,
				),
				nil,
			)
		}
		pathIndexByKey[entry.Path] = idx

		if previous, exists := fileIndexByKey[entry.File]; exists {
			return nil, faults.NewValidationError(
				fmt.Sprintf(
					"resourceInfo.externalizedAttributes[%d].file duplicates resourceInfo.externalizedAttributes[%d].file",
					idx,
					previous,
				),
				nil,
			)
		}
		fileIndexByKey[entry.File] = idx

		resolved = append(resolved, entry)
	}

	if len(resolved) == 0 {
		return nil, nil
	}

	return resolved, nil
}

func resolveExternalizedAttribute(item ExternalizedAttribute, idx int) (ResolvedExternalizedAttribute, error) {
	pathValue, err := normalizeExternalizedAttributePath(item.Path, idx)
	if err != nil {
		return ResolvedExternalizedAttribute{}, err
	}

	fileValue, err := normalizeExternalizedAttributeFile(item.File, idx)
	if err != nil {
		return ResolvedExternalizedAttribute{}, err
	}

	templateValue := strings.TrimSpace(item.Template)
	if templateValue == "" {
		templateValue = DefaultExternalizedAttributeTemplate
	}

	modeValue := strings.TrimSpace(item.Mode)
	if modeValue == "" {
		modeValue = ExternalizedAttributeModeText
	}
	if modeValue != ExternalizedAttributeModeText {
		return ResolvedExternalizedAttribute{}, faults.NewValidationError(
			fmt.Sprintf(
				"resourceInfo.externalizedAttributes[%d].mode %q is not supported",
				idx,
				item.Mode,
			),
			nil,
		)
	}

	saveBehaviorValue := strings.TrimSpace(item.SaveBehavior)
	if saveBehaviorValue == "" {
		saveBehaviorValue = ExternalizedAttributeSaveBehaviorExternalize
	}
	if saveBehaviorValue != ExternalizedAttributeSaveBehaviorExternalize {
		return ResolvedExternalizedAttribute{}, faults.NewValidationError(
			fmt.Sprintf(
				"resourceInfo.externalizedAttributes[%d].saveBehavior %q is not supported",
				idx,
				item.SaveBehavior,
			),
			nil,
		)
	}

	renderBehaviorValue := strings.TrimSpace(item.RenderBehavior)
	if renderBehaviorValue == "" {
		renderBehaviorValue = ExternalizedAttributeRenderBehaviorInclude
	}
	if renderBehaviorValue != ExternalizedAttributeRenderBehaviorInclude {
		return ResolvedExternalizedAttribute{}, faults.NewValidationError(
			fmt.Sprintf(
				"resourceInfo.externalizedAttributes[%d].renderBehavior %q is not supported",
				idx,
				item.RenderBehavior,
			),
			nil,
		)
	}

	return ResolvedExternalizedAttribute{
		Path:           pathValue,
		File:           fileValue,
		Template:       templateValue,
		Mode:           modeValue,
		SaveBehavior:   saveBehaviorValue,
		RenderBehavior: renderBehaviorValue,
		Enabled:        true,
	}, nil
}

func normalizeExternalizedAttributePath(value string, idx int) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", faults.NewValidationError(
			fmt.Sprintf("resourceInfo.externalizedAttributes[%d].path must not be empty", idx),
			nil,
		)
	}

	tokens, err := resource.ParseJSONPointer(trimmed)
	if err != nil {
		return "", faults.NewValidationError(
			fmt.Sprintf("resourceInfo.externalizedAttributes[%d].path must be a valid JSON pointer", idx),
			err,
		)
	}

	return resource.JSONPointerFromTokens(tokens), nil
}

func normalizeExternalizedAttributeFile(value string, idx int) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", faults.NewValidationError(
			fmt.Sprintf("resourceInfo.externalizedAttributes[%d].file must not be empty", idx),
			nil,
		)
	}
	if strings.HasPrefix(trimmed, "/") {
		return "", faults.NewValidationError(
			fmt.Sprintf("resourceInfo.externalizedAttributes[%d].file must stay within the resource directory", idx),
			nil,
		)
	}

	cleaned := path.Clean(trimmed)
	if cleaned == "." || cleaned == "" {
		return "", faults.NewValidationError(
			fmt.Sprintf("resourceInfo.externalizedAttributes[%d].file must not be empty", idx),
			nil,
		)
	}

	segments := strings.Split(cleaned, "/")
	for _, segment := range segments {
		if segment == ".." {
			return "", faults.NewValidationError(
				fmt.Sprintf("resourceInfo.externalizedAttributes[%d].file must stay within the resource directory", idx),
				nil,
			)
		}
	}

	return cleaned, nil
}
