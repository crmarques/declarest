package context

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

func resolveContextEnvPlaceholders(cfg *ContextConfig) error {
	if cfg == nil {
		return nil
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return err
	}

	if err := walkPlaceholderNode(&root); err != nil {
		return err
	}

	return root.Decode(cfg)
}

func walkPlaceholderNode(node *yaml.Node) error {
	if node == nil {
		return nil
	}

	switch node.Kind {
	case yaml.DocumentNode:
		for _, child := range node.Content {
			if err := walkPlaceholderNode(child); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			if err := walkPlaceholderNode(node.Content[i]); err != nil {
				return err
			}
			if err := walkPlaceholderNode(node.Content[i+1]); err != nil {
				return err
			}
		}
	case yaml.SequenceNode:
		for _, child := range node.Content {
			if err := walkPlaceholderNode(child); err != nil {
				return err
			}
		}
	case yaml.ScalarNode:
		if isStringScalar(node) && strings.Contains(node.Value, "${") {
			resolved, err := substituteEnvPlaceholders(node.Value)
			if err != nil {
				return err
			}
			node.Value = resolved
		}
	}
	return nil
}

func isStringScalar(node *yaml.Node) bool {
	return node != nil && node.Kind == yaml.ScalarNode && (node.Tag == "!!str" || node.Tag == "")
}

func substituteEnvPlaceholders(value string) (string, error) {
	if !strings.Contains(value, "${") {
		return value, nil
	}

	var builder strings.Builder
	for i := 0; i < len(value); {
		if value[i] == '$' && i+1 < len(value) && value[i+1] == '{' {
			start := i + 2
			end := strings.IndexByte(value[start:], '}')
			if end < 0 {
				return "", fmt.Errorf("missing closing brace in %q", value)
			}
			name := strings.TrimSpace(value[start : start+end])
			if name == "" {
				return "", fmt.Errorf("empty environment variable reference in %q", value)
			}
			envValue, ok := os.LookupEnv(name)
			if !ok {
				return "", fmt.Errorf("environment variable %q referenced in context is not set", name)
			}
			builder.WriteString(envValue)
			i = start + end + 1
			continue
		}
		builder.WriteByte(value[i])
		i++
	}
	return builder.String(), nil
}
