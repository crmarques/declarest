package openapi

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Spec struct {
	Paths []*PathItem
}

type PathItem struct {
	Template    string
	Segments    []string
	StaticCount int
	Operations  map[string]*Operation
}

type Operation struct {
	Method               string
	RequestContentTypes  []string
	ResponseContentTypes []string
}

func ParseSpec(data []byte) (*Spec, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, errors.New("openapi spec is empty")
	}

	var raw any
	var err error
	if looksLikeJSON(trimmed) {
		err = json.Unmarshal([]byte(trimmed), &raw)
	} else {
		err = yaml.Unmarshal([]byte(trimmed), &raw)
	}
	if err != nil {
		return nil, err
	}

	root, ok := normalizeValue(raw).(map[string]any)
	if !ok {
		return nil, errors.New("openapi spec must be a mapping")
	}

	pathsValue, ok := root["paths"].(map[string]any)
	if !ok {
		return nil, errors.New("openapi spec missing paths")
	}

	var items []*PathItem
	for template, value := range pathsValue {
		itemMap, ok := value.(map[string]any)
		if !ok {
			continue
		}
		operations := parseOperations(itemMap)
		if len(operations) == 0 {
			continue
		}
		segments := splitSegments(template)
		staticCount := 0
		for _, segment := range segments {
			if !isParamSegment(segment) {
				staticCount++
			}
		}
		items = append(items, &PathItem{
			Template:    template,
			Segments:    segments,
			StaticCount: staticCount,
			Operations:  operations,
		})
	}

	return &Spec{Paths: items}, nil
}

func (s *Spec) MatchPath(path string) *PathItem {
	if s == nil {
		return nil
	}
	segments := splitSegments(path)

	var best *PathItem
	for _, item := range s.Paths {
		if !segmentsMatch(item.Segments, segments) {
			continue
		}
		if best == nil {
			best = item
			continue
		}
		if item.StaticCount > best.StaticCount {
			best = item
			continue
		}
		if item.StaticCount == best.StaticCount {
			if len(item.Segments) > len(best.Segments) {
				best = item
				continue
			}
			if len(item.Segments) == len(best.Segments) && item.Template < best.Template {
				best = item
			}
		}
	}

	return best
}

func (p *PathItem) Operation(method string) *Operation {
	if p == nil {
		return nil
	}
	method = strings.ToLower(strings.TrimSpace(method))
	return p.Operations[method]
}

func looksLikeJSON(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	switch trimmed[0] {
	case '{', '[':
		return true
	default:
		return false
	}
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, val := range typed {
			out[key] = normalizeValue(val)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, val := range typed {
			strKey, ok := key.(string)
			if !ok {
				continue
			}
			out[strKey] = normalizeValue(val)
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, entry := range typed {
			out = append(out, normalizeValue(entry))
		}
		return out
	default:
		return typed
	}
}

func parseOperations(item map[string]any) map[string]*Operation {
	operations := make(map[string]*Operation)
	for key, value := range item {
		method := strings.ToLower(strings.TrimSpace(key))
		if !isHTTPMethod(method) {
			continue
		}
		op, ok := value.(map[string]any)
		if !ok {
			continue
		}
		reqTypes := requestContentTypes(op)
		respTypes := responseContentTypes(op)
		operations[method] = &Operation{
			Method:               method,
			RequestContentTypes:  reqTypes,
			ResponseContentTypes: respTypes,
		}
	}
	return operations
}

func requestContentTypes(op map[string]any) []string {
	requestBody, ok := op["requestBody"].(map[string]any)
	if !ok {
		return nil
	}
	content, ok := requestBody["content"].(map[string]any)
	if !ok {
		return nil
	}
	return sortedKeys(content)
}

func responseContentTypes(op map[string]any) []string {
	responses, ok := op["responses"].(map[string]any)
	if !ok {
		return nil
	}
	var preferred []string
	var fallback []string
	for code, entry := range responses {
		content := responseEntryContentTypes(entry)
		if len(content) == 0 {
			continue
		}
		if code == "default" {
			fallback = append(fallback, content...)
			continue
		}
		if strings.HasPrefix(code, "2") {
			preferred = append(preferred, content...)
		}
	}
	if len(preferred) > 0 {
		return uniqueSorted(preferred)
	}
	if len(fallback) > 0 {
		return uniqueSorted(fallback)
	}
	var anyTypes []string
	for _, entry := range responses {
		content := responseEntryContentTypes(entry)
		if len(content) > 0 {
			anyTypes = append(anyTypes, content...)
		}
	}
	return uniqueSorted(anyTypes)
}

func responseEntryContentTypes(entry any) []string {
	resp, ok := entry.(map[string]any)
	if !ok {
		return nil
	}
	content, ok := resp["content"].(map[string]any)
	if !ok {
		return nil
	}
	return sortedKeys(content)
}

func sortedKeys(value map[string]any) []string {
	if len(value) == 0 {
		return nil
	}
	keys := make([]string, 0, len(value))
	for key := range value {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	var unique []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	sort.Strings(unique)
	return unique
}

func isHTTPMethod(method string) bool {
	switch method {
	case "get", "post", "put", "patch", "delete", "head", "options":
		return true
	default:
		return false
	}
}

func splitSegments(path string) []string {
	trimmed := strings.TrimSpace(path)
	trimmed = strings.TrimSuffix(trimmed, "/")
	trimmed = strings.TrimPrefix(trimmed, "/")
	if trimmed == "" {
		return nil
	}
	parts := strings.Split(trimmed, "/")
	var segments []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segments = append(segments, part)
	}
	return segments
}

func segmentsMatch(pattern, actual []string) bool {
	if len(pattern) != len(actual) {
		return false
	}
	for idx, segment := range pattern {
		if isParamSegment(segment) {
			continue
		}
		if segment != actual[idx] {
			return false
		}
	}
	return true
}

func isParamSegment(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}
