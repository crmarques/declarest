package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type HeaderEntry struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

type HeaderList []string

func (h *HeaderList) UnmarshalJSON(data []byte) error {
	if h == nil {
		return nil
	}
	if len(bytes.TrimSpace(data)) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return nil
	}

	var asStrings []string
	if err := json.Unmarshal(data, &asStrings); err == nil {
		*h = HeaderList(asStrings)
		return nil
	}

	var asEntries []HeaderEntry
	if err := json.Unmarshal(data, &asEntries); err == nil {
		*h = headersFromEntries(asEntries)
		return nil
	}

	return fmt.Errorf("invalid httpHeaders format")
}

func (h *HeaderList) UnmarshalYAML(value *yaml.Node) error {
	if h == nil {
		return nil
	}
	if value == nil || value.Kind == 0 {
		return nil
	}

	var asStrings []string
	if err := value.Decode(&asStrings); err == nil {
		*h = HeaderList(asStrings)
		return nil
	}

	var asEntries []HeaderEntry
	if err := value.Decode(&asEntries); err == nil {
		*h = headersFromEntries(asEntries)
		return nil
	}

	return fmt.Errorf("invalid httpHeaders format")
}

func headersFromEntries(entries []HeaderEntry) HeaderList {
	var headers HeaderList
	for _, entry := range entries {
		name := strings.TrimSpace(entry.Name)
		value := strings.TrimSpace(entry.Value)
		if name == "" || value == "" {
			continue
		}
		headers = append(headers, fmt.Sprintf("%s: %s", name, value))
	}
	return headers
}

func SplitHeaderLine(header string) (string, string, bool) {
	parts := strings.SplitN(header, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	name := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if name == "" || value == "" {
		return "", "", false
	}
	return name, value, true
}

func HeaderMap(headers HeaderList) map[string][]string {
	result := make(map[string][]string)
	for _, header := range headers {
		name, value, ok := SplitHeaderLine(header)
		if !ok {
			continue
		}
		key := http.CanonicalHeaderKey(name)
		result[key] = append(result[key], value)
	}
	return result
}

func ApplyHeaderDefaults(headers map[string][]string, method string) map[string][]string {
	normalized := make(map[string][]string)
	for key, values := range headers {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		normalizedKey := http.CanonicalHeaderKey(key)
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			normalized[normalizedKey] = append(normalized[normalizedKey], value)
		}
	}

	if len(normalized["Accept"]) == 0 {
		normalized["Accept"] = []string{"application/json"}
	}

	if MethodSupportsBody(method) && len(normalized["Content-Type"]) == 0 {
		normalized["Content-Type"] = []string{"application/json"}
	}

	return normalized
}

func HeaderListFromMap(headers map[string][]string) HeaderList {
	if len(headers) == 0 {
		return nil
	}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var out HeaderList
	for _, key := range keys {
		values := headers[key]
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			out = append(out, fmt.Sprintf("%s: %s", key, value))
		}
	}
	return out
}

func EnsureHeaderDefaults(headers HeaderList, method string) HeaderList {
	return HeaderListFromMap(ApplyHeaderDefaults(HeaderMap(headers), method))
}
