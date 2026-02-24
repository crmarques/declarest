package metadata

import (
	"bytes"
	"encoding/json"

	"go.yaml.in/yaml/v3"
)

// stringListWire accepts either a JSON/YAML string or a list of strings and
// normalizes to a string slice.
type stringListWire []string

func (s *stringListWire) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		*s = nil
		return nil
	}

	if trimmed[0] == '"' {
		var single string
		if err := json.Unmarshal(trimmed, &single); err != nil {
			return err
		}
		*s = []string{single}
		return nil
	}

	var items []string
	if err := json.Unmarshal(trimmed, &items); err != nil {
		return err
	}
	*s = items
	return nil
}

func (s stringListWire) MarshalJSON() ([]byte, error) {
	return json.Marshal([]string(s))
}

func (s *stringListWire) UnmarshalYAML(value *yaml.Node) error {
	if value == nil {
		*s = nil
		return nil
	}
	if value.Kind == yaml.ScalarNode && value.Tag != "!!null" {
		var single string
		if err := value.Decode(&single); err != nil {
			return err
		}
		*s = []string{single}
		return nil
	}

	var items []string
	if err := value.Decode(&items); err != nil {
		return err
	}
	*s = items
	return nil
}

func (s stringListWire) MarshalYAML() (any, error) {
	if s == nil {
		return nil, nil
	}
	return []string(s), nil
}

func cloneStringListWire(values *stringListWire) []string {
	if values == nil {
		return nil
	}
	return cloneStringSlice([]string(*values))
}

func stringListWirePointer(values []string) *stringListWire {
	if values == nil {
		return nil
	}
	wire := stringListWire(cloneStringSlice(values))
	return &wire
}

func stringListWireNullPointer(values []string) *stringListWire {
	if values == nil {
		wire := stringListWire(nil)
		return &wire
	}
	return stringListWirePointer(values)
}
