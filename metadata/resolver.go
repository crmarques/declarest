package metadata

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/crmarques/declarest/resource"
)

type RelativePlaceholderResolver func(targetPath string) (map[string]any, bool)

type RenderOptions struct {
	RelativePlaceholderResolver RelativePlaceholderResolver
}

func DefaultMetadata(collectionSegments []string) resource.ResourceMetadata {
	collectionPath := collectionPathFromSegments(collectionSegments)
	resourceTemplate := defaultResourceTemplate()

	return resource.ResourceMetadata{
		ResourceInfo: &resource.ResourceInfoMetadata{
			IDFromAttribute:    "id",
			AliasFromAttribute: "id",
			CollectionPath:     collectionPath,
		},
		OperationInfo: &resource.OperationInfoMetadata{
			GetResource:    defaultOperationMetadata("GET", resourceTemplate),
			CreateResource: defaultOperationMetadata("POST", "."),
			UpdateResource: defaultOperationMetadata("PUT", resourceTemplate),
			DeleteResource: defaultOperationMetadata("DELETE", resourceTemplate),
			ListCollection: defaultOperationMetadata("GET", "."),
			CompareResources: &resource.CompareMetadata{
				IgnoreAttributes: []string{},
			},
		},
	}
}

func MergeMetadata(base, overrides resource.ResourceMetadata) resource.ResourceMetadata {
	result := base

	if overrides.ResourceInfo != nil {
		if result.ResourceInfo == nil {
			result.ResourceInfo = &resource.ResourceInfoMetadata{}
		}
		result.ResourceInfo.IDFromAttribute = chooseString(result.ResourceInfo.IDFromAttribute, overrides.ResourceInfo.IDFromAttribute)
		result.ResourceInfo.AliasFromAttribute = chooseString(result.ResourceInfo.AliasFromAttribute, overrides.ResourceInfo.AliasFromAttribute)
		result.ResourceInfo.CollectionPath = chooseString(result.ResourceInfo.CollectionPath, overrides.ResourceInfo.CollectionPath)
		if overrides.ResourceInfo.SecretInAttributes != nil {
			result.ResourceInfo.SecretInAttributes = append([]string{}, overrides.ResourceInfo.SecretInAttributes...)
		}
	}

	if overrides.OperationInfo != nil {
		if result.OperationInfo == nil {
			result.OperationInfo = &resource.OperationInfoMetadata{}
		}
		result.OperationInfo.GetResource = mergeOperationMetadata(result.OperationInfo.GetResource, overrides.OperationInfo.GetResource)
		result.OperationInfo.CreateResource = mergeOperationMetadata(result.OperationInfo.CreateResource, overrides.OperationInfo.CreateResource)
		result.OperationInfo.UpdateResource = mergeOperationMetadata(result.OperationInfo.UpdateResource, overrides.OperationInfo.UpdateResource)
		result.OperationInfo.DeleteResource = mergeOperationMetadata(result.OperationInfo.DeleteResource, overrides.OperationInfo.DeleteResource)
		result.OperationInfo.ListCollection = mergeOperationMetadata(result.OperationInfo.ListCollection, overrides.OperationInfo.ListCollection)
		result.OperationInfo.CompareResources = mergeCompareMetadata(result.OperationInfo.CompareResources, overrides.OperationInfo.CompareResources)
	}

	return result
}

func RenderTemplates(meta resource.ResourceMetadata, resourcePath string, ctx map[string]any, opts RenderOptions) resource.ResourceMetadata {
	isCollection := resource.IsCollectionPath(resourcePath)

	if meta.ResourceInfo != nil && !isCollection {
		if idAttr := strings.TrimSpace(meta.ResourceInfo.IDFromAttribute); idAttr != "" {
			if value, ok := resource.GetAttrPath(ctx, idAttr); ok {
				ctx["id"] = value
			}
		}
		if aliasAttr := strings.TrimSpace(meta.ResourceInfo.AliasFromAttribute); aliasAttr != "" {
			if value, ok := resource.GetAttrPath(ctx, aliasAttr); ok {
				ctx["alias"] = value
			}
		}
	}

	if isCollection {
		ctx["id"] = "{{.id}}"
		ctx["alias"] = "{{.alias}}"
	} else {
		if _, ok := ctx["id"]; !ok {
			ctx["id"] = resource.LastSegment(resourcePath)
		}
		if _, ok := ctx["alias"]; !ok {
			ctx["alias"] = resource.LastSegment(resourcePath)
		}
	}

	var collectionPath string
	if meta.ResourceInfo != nil {
		meta.ResourceInfo.CollectionPath = renderString(meta.ResourceInfo.CollectionPath, ctx, resourcePath, opts)
		collectionPath = meta.ResourceInfo.CollectionPath
	}

	pathCtx := cloneContext(ctx)
	if !isCollection {
		pathCtx["id"] = "{{.id}}"
		pathCtx["alias"] = "{{.alias}}"
	}

	if meta.OperationInfo != nil {
		meta.OperationInfo.GetResource = renderOperation(meta.OperationInfo.GetResource, ctx, pathCtx, collectionPath, resourcePath, opts)
		meta.OperationInfo.CreateResource = renderOperation(meta.OperationInfo.CreateResource, ctx, pathCtx, collectionPath, resourcePath, opts)
		meta.OperationInfo.UpdateResource = renderOperation(meta.OperationInfo.UpdateResource, ctx, pathCtx, collectionPath, resourcePath, opts)
		meta.OperationInfo.DeleteResource = renderOperation(meta.OperationInfo.DeleteResource, ctx, pathCtx, collectionPath, resourcePath, opts)
		meta.OperationInfo.ListCollection = renderOperation(meta.OperationInfo.ListCollection, ctx, pathCtx, collectionPath, resourcePath, opts)
		applyDefaultHeaders(meta.OperationInfo)
	}

	return meta
}

func defaultOperationMetadata(method, path string) *resource.OperationMetadata {
	return &resource.OperationMetadata{
		HTTPMethod: method,
		URL: &resource.OperationURLMetadata{
			Path: path,
		},
		HTTPHeaders: defaultHeadersFor(method),
	}
}

func defaultResourceTemplate() string {
	return "./{{.id}}"
}

func collectionPathFromSegments(segments []string) string {
	if len(segments) == 0 {
		return "/"
	}
	return resource.NormalizePath("/" + strings.Join(segments, "/"))
}

func mergeOperationMetadata(base, override *resource.OperationMetadata) *resource.OperationMetadata {
	if override == nil {
		return base
	}
	if base == nil {
		return cloneOperationMetadata(override)
	}

	base.HTTPMethod = chooseString(base.HTTPMethod, override.HTTPMethod)
	if override.HTTPHeaders != nil {
		base.HTTPHeaders = append(resource.HeaderList{}, override.HTTPHeaders...)
	}
	if override.URL != nil {
		if base.URL == nil {
			base.URL = &resource.OperationURLMetadata{}
		}
		base.URL.Path = chooseString(base.URL.Path, override.URL.Path)
		if override.URL.QueryStrings != nil {
			base.URL.QueryStrings = append([]string{}, override.URL.QueryStrings...)
		}
	}
	if override.Payload != nil {
		base.Payload = clonePayloadConfig(override.Payload)
	}
	base.JQFilter = chooseString(base.JQFilter, override.JQFilter)
	return base
}

func mergeCompareMetadata(base, override *resource.CompareMetadata) *resource.CompareMetadata {
	if override == nil {
		return base
	}
	if base == nil {
		return cloneCompareMetadata(override)
	}

	if override.IgnoreAttributes != nil {
		base.IgnoreAttributes = append([]string{}, override.IgnoreAttributes...)
	}
	if override.SuppressAttributes != nil {
		base.SuppressAttributes = append([]string{}, override.SuppressAttributes...)
	}
	if override.FilterAttributes != nil {
		base.FilterAttributes = append([]string{}, override.FilterAttributes...)
	}
	base.JQExpression = chooseString(base.JQExpression, override.JQExpression)
	return base
}

func chooseString(current, candidate string) string {
	if strings.TrimSpace(candidate) != "" {
		return candidate
	}
	return current
}

func cloneOperationMetadata(src *resource.OperationMetadata) *resource.OperationMetadata {
	if src == nil {
		return nil
	}
	result := &resource.OperationMetadata{
		HTTPMethod:  src.HTTPMethod,
		HTTPHeaders: append(resource.HeaderList{}, src.HTTPHeaders...),
		JQFilter:    src.JQFilter,
	}
	if src.URL != nil {
		result.URL = &resource.OperationURLMetadata{
			Path:         src.URL.Path,
			QueryStrings: append([]string{}, src.URL.QueryStrings...),
		}
	}
	if src.Payload != nil {
		result.Payload = clonePayloadConfig(src.Payload)
	}
	return result
}

func clonePayloadConfig(src *resource.OperationPayloadConfig) *resource.OperationPayloadConfig {
	if src == nil {
		return nil
	}
	return &resource.OperationPayloadConfig{
		SuppressAttributes: append([]string{}, src.SuppressAttributes...),
		FilterAttributes:   append([]string{}, src.FilterAttributes...),
		JQExpression:       src.JQExpression,
	}
}

func cloneCompareMetadata(src *resource.CompareMetadata) *resource.CompareMetadata {
	if src == nil {
		return nil
	}
	return &resource.CompareMetadata{
		IgnoreAttributes:   append([]string{}, src.IgnoreAttributes...),
		SuppressAttributes: append([]string{}, src.SuppressAttributes...),
		FilterAttributes:   append([]string{}, src.FilterAttributes...),
		JQExpression:       src.JQExpression,
	}
}

func renderOperation(op *resource.OperationMetadata, ctx map[string]any, pathCtx map[string]any, collectionPath, resourcePath string, opts RenderOptions) *resource.OperationMetadata {
	if op == nil {
		return op
	}
	if pathCtx == nil {
		pathCtx = ctx
	}
	if op.URL != nil {
		op.URL.Path = resolveOperationPath(renderString(op.URL.Path, pathCtx, resourcePath, opts), collectionPath)
		if len(op.URL.QueryStrings) > 0 {
			for idx, value := range op.URL.QueryStrings {
				op.URL.QueryStrings[idx] = renderString(value, ctx, resourcePath, opts)
			}
		}
	}
	if len(op.HTTPHeaders) > 0 {
		op.HTTPHeaders = renderHeaders(op.HTTPHeaders, pathCtx, resourcePath, opts)
	}
	if strings.TrimSpace(op.JQFilter) != "" {
		op.JQFilter = renderString(op.JQFilter, ctx, resourcePath, opts)
	}
	return op
}

func applyDefaultHeaders(info *resource.OperationInfoMetadata) {
	if info == nil {
		return
	}
	info.GetResource = ensureHeaders(info.GetResource)
	info.CreateResource = ensureHeaders(info.CreateResource)
	info.UpdateResource = ensureHeaders(info.UpdateResource)
	info.DeleteResource = ensureHeaders(info.DeleteResource)
	info.ListCollection = ensureHeaders(info.ListCollection)
}

func ensureHeaders(op *resource.OperationMetadata) *resource.OperationMetadata {
	if op == nil {
		return op
	}
	op.HTTPHeaders = resource.EnsureHeaderDefaults(op.HTTPHeaders, op.HTTPMethod)
	return op
}

func defaultHeadersFor(method string) resource.HeaderList {
	headers := resource.HeaderList{"Accept: application/json"}
	if resource.MethodSupportsBody(method) {
		headers = append(headers, "Content-Type: application/json")
	}
	return headers
}

func renderString(raw string, ctx map[string]any, resourcePath string, opts RenderOptions) string {
	if strings.TrimSpace(raw) == "" || !strings.Contains(raw, "{{") {
		return raw
	}

	rendered := replaceRelativePlaceholders(raw, resourcePath, opts.RelativePlaceholderResolver)

	tmpl, err := template.New("metadata").
		Option("missingkey=default").
		Parse(rendered)
	if err != nil {
		return rendered
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return rendered
	}
	return buf.String()
}

func renderHeaders(headers resource.HeaderList, ctx map[string]any, resourcePath string, opts RenderOptions) resource.HeaderList {
	rendered := make(resource.HeaderList, 0, len(headers))
	for _, header := range headers {
		name, value, ok := resource.SplitHeaderLine(header)
		if !ok {
			continue
		}
		value = renderString(value, ctx, resourcePath, opts)
		rendered = append(rendered, fmt.Sprintf("%s: %s", name, value))
	}
	return rendered
}

func cloneContext(src map[string]any) map[string]any {
	if src == nil {
		return map[string]any{}
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

func replaceRelativePlaceholders(raw, resourcePath string, resolver RelativePlaceholderResolver) string {
	if resolver == nil || !strings.Contains(raw, "{{../") {
		return raw
	}

	var (
		builder strings.Builder
		start   int
	)

	for {
		open := strings.Index(raw[start:], "{{")
		if open == -1 {
			builder.WriteString(raw[start:])
			break
		}
		open += start
		close := strings.Index(raw[open:], "}}")
		if close == -1 {
			builder.WriteString(raw[start:])
			break
		}
		close += open

		builder.WriteString(raw[start:open])

		placeholder := strings.TrimSpace(raw[open+2 : close])
		if value, ok := resolveRelativePlaceholder(placeholder, resourcePath, resolver); ok {
			builder.WriteString(value)
		} else {
			builder.WriteString(raw[open : close+2])
		}

		start = close + 2
	}

	return builder.String()
}

func resolveRelativePlaceholder(expr, resourcePath string, resolver RelativePlaceholderResolver) (string, bool) {
	expr = strings.TrimSpace(expr)
	if !strings.HasPrefix(expr, "../") {
		return "", false
	}

	up := 0
	for strings.HasPrefix(expr, "../") {
		up++
		expr = strings.TrimPrefix(expr, "../")
	}

	expr = strings.TrimPrefix(expr, "./")
	attrPath := strings.TrimPrefix(expr, ".")
	attrPath = strings.TrimSpace(attrPath)
	if attrPath == "" {
		return "", false
	}

	segments := resource.SplitPathSegments(resourcePath)
	if resource.IsCollectionPath(resourcePath) {
		segments = append(segments, "")
	}
	if len(segments) == 0 {
		return "", false
	}
	if up > len(segments) {
		return "", false
	}

	targetSegments := segments[:len(segments)-up]
	if len(targetSegments) == 0 {
		return "", false
	}

	targetPath := "/" + strings.Join(targetSegments, "/")
	attrs, ok := resolver(targetPath)
	if !ok {
		return "", false
	}

	value, ok := resource.GetAttrPath(attrs, attrPath)
	if !ok {
		return "", false
	}

	return fmt.Sprint(value), true
}

func resolveOperationPath(raw string, collectionPath string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return trimmed
	}

	if strings.HasPrefix(trimmed, "./") {
		base := strings.TrimSpace(collectionPath)
		if base == "" {
			base = "/"
		}
		suffix := strings.TrimPrefix(trimmed, ".")
		if !strings.HasPrefix(suffix, "/") {
			suffix = "/" + suffix
		}
		return resource.NormalizePath(strings.TrimRight(base, "/") + suffix)
	}

	if strings.HasPrefix(trimmed, "/") {
		return resource.NormalizePath(trimmed)
	}

	return trimmed
}
