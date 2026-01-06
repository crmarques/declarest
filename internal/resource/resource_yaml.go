package resource

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

func NewResourceFromYAML(data []byte) (Resource, error) {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Resource{}, err
	}
	normalized, err := normalizeYAMLValue(raw)
	if err != nil {
		return Resource{}, err
	}
	return NewResource(normalized)
}

func (r Resource) MarshalYAMLBytes() ([]byte, error) {
	payload, err := r.MarshalJSON()
	if err != nil {
		return nil, err
	}

	var raw any
	if err := yaml.Unmarshal(payload, &raw); err != nil {
		return nil, fmt.Errorf("failed to convert resource to yaml: %w", err)
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return out, nil
	}
	if out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return out, nil
}

func normalizeYAMLValue(value any) (any, error) {
	switch t := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(t))
		for key, val := range t {
			normalized, err := normalizeYAMLValue(val)
			if err != nil {
				return nil, err
			}
			out[key] = normalized
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(t))
		for key, val := range t {
			ks, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("yaml key %v is not a string", key)
			}
			normalized, err := normalizeYAMLValue(val)
			if err != nil {
				return nil, err
			}
			out[ks] = normalized
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for i, item := range t {
			normalized, err := normalizeYAMLValue(item)
			if err != nil {
				return nil, err
			}
			out[i] = normalized
		}
		return out, nil
	default:
		return t, nil
	}
}
