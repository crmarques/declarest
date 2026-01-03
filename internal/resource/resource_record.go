package resource

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/itchyny/gojq"
)

func (rr ResourceRecord) IsCollection(path string) bool {
	return IsCollectionPath(path)
}

func (rr ResourceRecord) RemoteResourcePath(res Resource) string {
	collection := strings.TrimRight(rr.CollectionPath(), "/")

	if rr.Meta.ResourceInfo != nil {
		if idAttr := strings.TrimSpace(rr.Meta.ResourceInfo.IDFromAttribute); idAttr != "" {
			if value, ok := LookupValueFromResource(res, idAttr); ok && value != "" {
				return NormalizePath(collection + "/" + sanitizePathSegment(value))
			}
		}
	}

	fallback := LastSegment(rr.Path)
	return NormalizePath(collection + "/" + fallback)
}

func (rr ResourceRecord) AliasPath(res Resource) string {
	path := rr.Path
	info := rr.Meta.ResourceInfo
	if info == nil {
		return path
	}
	aliasAttr := strings.TrimSpace(info.AliasFromAttribute)
	if aliasAttr == "" {
		return path
	}

	value, ok := LookupValueFromResource(res, aliasAttr)
	if !ok {
		return path
	}

	value = sanitizePathSegment(value)
	if value == "" {
		return path
	}

	// Alias paths should be based on the local record path, not on the remote
	// collection path from metadata overrides. This lets repositories keep a
	// domain-friendly structure even when remote endpoints differ.
	base := strings.TrimSpace(rr.Path)
	if base == "" {
		base = "/"
	}
	isCollection := IsCollectionPath(base)
	base = NormalizePath(base)
	if !isCollection {
		last := LastSegment(base)
		if last != "" {
			base = NormalizePath(strings.TrimSuffix(base, "/"+last))
		}
	}
	base = strings.TrimRight(base, "/")
	if base == "" || base == "/" {
		return "/" + value
	}
	return NormalizePath(base + "/" + value)
}

func (rr ResourceRecord) ReadOperation(isCollection bool) *OperationMetadata {
	if rr.Meta.OperationInfo == nil {
		return nil
	}
	if isCollection {
		if rr.Meta.OperationInfo.ListCollection != nil {
			return rr.Meta.OperationInfo.ListCollection
		}
		return rr.Meta.OperationInfo.GetResource
	}
	if rr.Meta.OperationInfo.GetResource != nil {
		return rr.Meta.OperationInfo.GetResource
	}
	return nil
}

func (rr ResourceRecord) CreateOperation() *OperationMetadata {
	if rr.Meta.OperationInfo == nil {
		return nil
	}
	if rr.Meta.OperationInfo.CreateResource != nil {
		return rr.Meta.OperationInfo.CreateResource
	}
	if rr.Meta.OperationInfo.UpdateResource != nil {
		return rr.Meta.OperationInfo.UpdateResource
	}
	return rr.Meta.OperationInfo.GetResource
}

func (rr ResourceRecord) UpdateOperation() *OperationMetadata {
	if rr.Meta.OperationInfo == nil {
		return nil
	}
	if rr.Meta.OperationInfo.UpdateResource != nil {
		return rr.Meta.OperationInfo.UpdateResource
	}
	return rr.Meta.OperationInfo.GetResource
}

func (rr ResourceRecord) DeleteOperation() *OperationMetadata {
	if rr.Meta.OperationInfo == nil {
		return nil
	}
	if rr.Meta.OperationInfo.DeleteResource != nil {
		return rr.Meta.OperationInfo.DeleteResource
	}
	return rr.Meta.OperationInfo.GetResource
}

func (rr ResourceRecord) ReadPayload() *OperationPayloadConfig {
	if rr.Meta.OperationInfo == nil || rr.Meta.OperationInfo.GetResource == nil {
		return nil
	}
	return rr.Meta.OperationInfo.GetResource.Payload
}

func (rr ResourceRecord) ListPayload() *OperationPayloadConfig {
	if rr.Meta.OperationInfo == nil || rr.Meta.OperationInfo.ListCollection == nil {
		return nil
	}
	return rr.Meta.OperationInfo.ListCollection.Payload
}

func (rr ResourceRecord) OperationPayload(op *OperationMetadata) *OperationPayloadConfig {
	if op == nil {
		return nil
	}
	return op.Payload
}

func (rr ResourceRecord) ResolveOperationPath(resourcePath string, op *OperationMetadata, isCollection bool) (string, error) {
	templatePath := ""
	if op != nil && op.URL != nil {
		templatePath = strings.TrimSpace(op.URL.Path)
	}
	if templatePath == "" {
		return resourcePath, nil
	}

	context := rr.buildTemplateContext(resourcePath, isCollection)
	rendered, err := executeTemplate(templatePath, context)
	if err != nil {
		return "", err
	}

	rendered = strings.TrimSpace(rendered)
	if rendered == "." {
		return rr.CollectionPath(), nil
	}
	if strings.HasPrefix(rendered, "./") {
		base := rr.CollectionPath()
		suffix := strings.TrimPrefix(rendered, ".")
		if !strings.HasPrefix(suffix, "/") {
			suffix = "/" + suffix
		}
		return NormalizePath(strings.TrimRight(base, "/") + suffix), nil
	}

	if strings.HasPrefix(rendered, "/") {
		return NormalizePath(rendered), nil
	}

	return rendered, nil
}

func (rr ResourceRecord) HeadersFor(op *OperationMetadata, resourcePath string, isCollection bool) map[string][]string {
	var headers HeaderList
	if op != nil {
		headers = op.HTTPHeaders
	}

	ctx := rr.buildTemplateContext(resourcePath, isCollection)
	rendered := make(HeaderList, 0, len(headers))
	for _, header := range headers {
		name, value, ok := SplitHeaderLine(header)
		if !ok {
			continue
		}
		if strings.Contains(value, "{{") {
			if resolved, err := executeTemplate(value, ctx); err == nil {
				value = resolved
			}
		}
		rendered = append(rendered, fmt.Sprintf("%s: %s", name, value))
	}

	method := ""
	if op != nil {
		method = op.HTTPMethod
	}
	return ApplyHeaderDefaults(HeaderMap(rendered), method)
}

func (rr ResourceRecord) QueryFor(op *OperationMetadata) map[string][]string {
	result := map[string][]string{}
	if op == nil || op.URL == nil {
		return result
	}
	for _, entry := range op.URL.QueryStrings {
		parts := strings.SplitN(entry, "=", 2)
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		value := ""
		if len(parts) == 2 {
			value = strings.TrimSpace(parts[1])
		}
		result[key] = append(result[key], value)
	}
	return result
}

func (rr ResourceRecord) ApplyPayload(res Resource, payload *OperationPayloadConfig) (Resource, error) {
	if payload == nil {
		return res, nil
	}

	current := res.Clone()

	if len(payload.FilterAttributes) > 0 {
		if obj, ok := current.AsObject(); ok {
			current.V = filterAttributes(obj, payload.FilterAttributes)
		}
	}

	if len(payload.SuppressAttributes) > 0 {
		if obj, ok := current.AsObject(); ok {
			current.V = suppressAttributes(obj, payload.SuppressAttributes)
		}
	}

	expr := strings.TrimSpace(payload.JQExpression)
	if expr != "" {
		value, err := executeJQ(current.V, expr)
		if err != nil {
			return Resource{}, err
		}
		normalized, err := NewResource(value)
		if err != nil {
			return Resource{}, err
		}
		return normalized, nil
	}

	return current, nil
}

func (rr ResourceRecord) buildTemplateContext(resourcePath string, isCollection bool) map[string]any {
	ctx := make(map[string]any)
	ctx["path"] = resourcePath
	ctx["collection"] = isCollection

	if rr.Meta.ResourceInfo != nil {
		ctx["collectionPath"] = rr.Meta.ResourceInfo.CollectionPath
		if idAttr := strings.TrimSpace(rr.Meta.ResourceInfo.IDFromAttribute); idAttr != "" {
			ctx["idAttribute"] = idAttr
			if value, ok := LookupValueFromResource(rr.Data, idAttr); ok && value != "" {
				ctx["id"] = value
			}
		}
		if aliasAttr := strings.TrimSpace(rr.Meta.ResourceInfo.AliasFromAttribute); aliasAttr != "" {
			ctx["aliasAttribute"] = aliasAttr
			if value, ok := LookupValueFromResource(rr.Data, aliasAttr); ok && value != "" {
				ctx["alias"] = value
			}
		}
	}

	if _, ok := ctx["id"]; !ok {
		ctx["id"] = LastSegment(resourcePath)
	}
	if _, ok := ctx["alias"]; !ok {
		ctx["alias"] = LastSegment(resourcePath)
	}

	return ctx
}

func executeTemplate(tmpl string, ctx map[string]any) (string, error) {
	parsed, err := template.New("resource-record-path").Option("missingkey=default").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := parsed.Execute(&buf, ctx); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func executeJQ(input any, expression string) (any, error) {
	query, err := gojq.Parse(expression)
	if err != nil {
		return nil, err
	}
	iter := query.Run(input)

	var results []any
	for {
		value, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := value.(error); ok {
			return nil, err
		}
		results = append(results, value)
	}

	if len(results) == 0 {
		return nil, nil
	}
	if len(results) == 1 {
		return results[0], nil
	}
	return results, nil
}

func filterAttributes(src map[string]any, paths []string) map[string]any {
	result := make(map[string]any, len(paths))
	for _, path := range paths {
		if value, ok := GetAttrPath(src, path); ok {
			setAttrPath(result, path, value)
		}
	}
	return result
}

func suppressAttributes(src map[string]any, paths []string) map[string]any {
	cloned := cloneMap(src)
	for _, path := range paths {
		deleteAttrPath(cloned, path)
	}
	return cloned
}

// GetAttrPath retrieves a nested value from a map using dot-separated segments.
func GetAttrPath(obj map[string]any, path string) (any, bool) {
	segments := splitAttrPath(path)
	if len(segments) == 0 {
		return nil, false
	}
	current := obj
	for idx, segment := range segments {
		value, ok := current[segment]
		if !ok {
			return nil, false
		}
		if idx == len(segments)-1 {
			return value, true
		}
		next, ok := value.(map[string]any)
		if !ok {
			return nil, false
		}
		current = next
	}
	return nil, false
}

func setAttrPath(obj map[string]any, path string, value any) {
	segments := splitAttrPath(path)
	if len(segments) == 0 {
		return
	}
	current := obj
	for idx, segment := range segments {
		if idx == len(segments)-1 {
			current[segment] = value
			return
		}
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[segment] = next
		}
		current = next
	}
}

func deleteAttrPath(obj map[string]any, path string) {
	segments := splitAttrPath(path)
	if len(segments) == 0 {
		return
	}
	current := obj
	for idx, segment := range segments {
		if idx == len(segments)-1 {
			delete(current, segment)
			return
		}
		next, ok := current[segment].(map[string]any)
		if !ok {
			return
		}
		current = next
	}
}

func cloneMap(src map[string]any) map[string]any {
	result := make(map[string]any, len(src))
	for key, value := range src {
		if nested, ok := value.(map[string]any); ok {
			result[key] = cloneMap(nested)
			continue
		}
		result[key] = value
	}
	return result
}

func splitAttrPath(path string) []string {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil
	}
	parts := strings.Split(path, ".")
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

func LookupValueFromResource(res Resource, path string) (string, bool) {
	obj, ok := res.AsObject()
	if !ok {
		return "", false
	}
	value, ok := GetAttrPath(obj, path)
	if !ok {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return v, true
	case fmt.Stringer:
		return v.String(), true
	default:
		return fmt.Sprint(v), true
	}
}

func sanitizePathSegment(segment string) string {
	segment = strings.TrimSpace(segment)
	segment = strings.ReplaceAll(segment, "/", "-")
	segment = strings.ReplaceAll(segment, "\\", "-")
	return segment
}

func LastSegment(path string) string {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, "/")
	return parts[len(parts)-1]
}

func NormalizePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	for len(path) > 1 && strings.HasSuffix(path, "/") {
		path = strings.TrimSuffix(path, "/")
	}
	return path
}

func IsCollectionPath(path string) bool {
	return strings.HasSuffix(strings.TrimSpace(path), "/")
}
