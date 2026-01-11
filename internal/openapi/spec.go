package openapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Spec struct {
	Paths           []*PathItem
	components      map[string]map[string]any
	parameterValues map[string]map[string]any
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
	RequestSchema        map[string]any
	HeaderParameters     map[string]string
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

	var components map[string]map[string]any
	var parameterValues map[string]map[string]any
	if compValue, ok := root["components"].(map[string]any); ok {
		if schemas, ok := compValue["schemas"].(map[string]any); ok {
			components = make(map[string]map[string]any, len(schemas))
			for key, entry := range schemas {
				if schemaMap, ok := entry.(map[string]any); ok {
					components[key] = schemaMap
				}
			}
		}
		if params, ok := compValue["parameters"].(map[string]any); ok {
			parameterValues = make(map[string]map[string]any, len(params))
			for key, entry := range params {
				if paramMap, ok := entry.(map[string]any); ok {
					parameterValues[key] = paramMap
				}
			}
		}
	}

	var items []*PathItem
	spec := &Spec{components: components, parameterValues: parameterValues}
	for template, value := range pathsValue {
		itemMap, ok := value.(map[string]any)
		if !ok {
			continue
		}
		operations := parseOperations(itemMap, spec)
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

	spec.Paths = items
	return spec, nil
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

func parseOperations(item map[string]any, spec *Spec) map[string]*Operation {
	operations := make(map[string]*Operation)
	pathHeaders := headerParametersFromList(item["parameters"], spec)
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
		opHeaders := mergeHeaderParameters(pathHeaders, headerParametersFromList(op["parameters"], spec))
		operations[method] = &Operation{
			Method:               method,
			RequestContentTypes:  reqTypes,
			ResponseContentTypes: respTypes,
			RequestSchema:        requestSchema(op, spec),
			HeaderParameters:     opHeaders,
		}
	}
	return operations
}

func requestSchema(op map[string]any, spec *Spec) map[string]any {
	if spec == nil {
		return nil
	}
	requestBody, ok := op["requestBody"].(map[string]any)
	if !ok {
		return nil
	}
	content, ok := requestBody["content"].(map[string]any)
	if !ok {
		return nil
	}

	for _, media := range []string{"application/json", "application/merge-patch+json", "application/json-patch+json"} {
		if entry, ok := content[media]; ok {
			entryMap, ok := entry.(map[string]any)
			if !ok {
				continue
			}
			if schema, ok := spec.schemaFromContent(entryMap); ok {
				return schema
			}
		}
	}

	for _, entry := range content {
		entryMap, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if schema, ok := spec.schemaFromContent(entryMap); ok {
			return schema
		}
	}

	return nil
}

func headerParametersFromList(value any, spec *Spec) map[string]string {
	params := parseParameterList(value, spec)
	if len(params) == 0 {
		return nil
	}
	values := make(map[string]string)
	for _, param := range params {
		in, ok := param["in"].(string)
		if !ok || strings.ToLower(strings.TrimSpace(in)) != "header" {
			continue
		}
		nameValue, ok := param["name"].(string)
		if !ok {
			continue
		}
		name := strings.TrimSpace(nameValue)
		if name == "" {
			continue
		}
		if value, ok := headerValueFromParameter(param, spec); ok {
			values[name] = value
		}
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

func parseParameterList(value any, spec *Spec) []map[string]any {
	rawList, ok := value.([]any)
	if !ok {
		return nil
	}
	var result []map[string]any
	for _, entry := range rawList {
		param, ok := spec.resolveParameter(entry)
		if !ok || param == nil {
			continue
		}
		result = append(result, param)
	}
	return result
}

func mergeHeaderParameters(base, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	merged := make(map[string]string)
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if strings.TrimSpace(value) == "" {
			continue
		}
		merged[key] = value
	}
	if len(merged) == 0 {
		return nil
	}
	return merged
}

func headerValueFromParameter(param map[string]any, spec *Spec) (string, bool) {
	if example, ok := param["example"]; ok {
		if str, ok := valueToString(example); ok {
			return str, true
		}
	}
	if schemaValue, ok := param["schema"]; ok {
		if schema, ok := spec.resolveSchema(schemaValue); ok {
			if def, ok := DefaultValueForSchema(spec, schema); ok {
				if str, ok := valueToString(def); ok {
					return str, true
				}
			}
		}
	}
	if def, ok := param["default"]; ok {
		if str, ok := valueToString(def); ok {
			return str, true
		}
	}
	return "", false
}

func valueToString(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	switch typed := value.(type) {
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	case fmt.Stringer:
		trimmed := strings.TrimSpace(typed.String())
		if trimmed == "" {
			return "", false
		}
		return trimmed, true
	default:
		normalized := strings.TrimSpace(fmt.Sprint(typed))
		if normalized == "" {
			return "", false
		}
		return normalized, true
	}
}

func (s *Spec) schemaFromContent(entry map[string]any) (map[string]any, bool) {
	if entry == nil {
		return nil, false
	}
	value, ok := entry["schema"]
	if !ok {
		return nil, false
	}
	return s.resolveSchema(value)
}

func (s *Spec) resolveSchema(value any) (map[string]any, bool) {
	if value == nil {
		return nil, false
	}
	node, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	if ref, ok := node["$ref"].(string); ok {
		return s.schemaByRef(ref)
	}
	return node, true
}

func (s *Spec) schemaByRef(ref string) (map[string]any, bool) {
	if s == nil || s.components == nil || !strings.HasPrefix(ref, "#/components/schemas/") {
		return nil, false
	}
	name := strings.TrimPrefix(ref, "#/components/schemas/")
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false
	}
	schema, ok := s.components[name]
	return schema, ok
}

func (s *Spec) resolveParameter(value any) (map[string]any, bool) {
	node, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	if ref, ok := node["$ref"].(string); ok {
		return s.parameterByRef(ref)
	}
	return node, true
}

func (s *Spec) parameterByRef(ref string) (map[string]any, bool) {
	if s == nil || s.parameterValues == nil {
		return nil, false
	}
	const prefix = "#/components/parameters/"
	if !strings.HasPrefix(ref, prefix) {
		return nil, false
	}
	name := strings.TrimPrefix(ref, prefix)
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, false
	}
	param, ok := s.parameterValues[name]
	return param, ok
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
