package repository

import (
	"fmt"
	"strings"
)

type ResourceFormat string

const (
	ResourceFormatJSON ResourceFormat = "json"
	ResourceFormatYAML ResourceFormat = "yaml"
)

const (
	resourceFileJSON = "resource.json"
	resourceFileYAML = "resource.yaml"
	resourceFileYML  = "resource.yml"
)

func ParseResourceFormat(raw string) (ResourceFormat, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "", "json":
		return ResourceFormatJSON, nil
	case "yaml", "yml":
		return ResourceFormatYAML, nil
	default:
		return "", fmt.Errorf("unsupported resource format %q (expected json or yaml)", raw)
	}
}

func normalizeResourceFormat(format ResourceFormat) ResourceFormat {
	normalized := strings.ToLower(strings.TrimSpace(string(format)))
	switch normalized {
	case "yaml", "yml":
		return ResourceFormatYAML
	default:
		return ResourceFormatJSON
	}
}

func resourceFileNameForFormat(format ResourceFormat) string {
	switch normalizeResourceFormat(format) {
	case ResourceFormatYAML:
		return resourceFileYAML
	default:
		return resourceFileJSON
	}
}
