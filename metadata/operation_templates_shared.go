package metadata

// RenderTemplateString exposes the metadata package's canonical template string
// rendering semantics for sibling packages such as metadata/render.
func RenderTemplateString(field string, raw string, scope map[string]any) (string, error) {
	return renderTemplateString(field, raw, scope)
}

// NormalizeRenderedOperationPath exposes the canonical rendered path
// normalization used by operation spec resolution.
func NormalizeRenderedOperationPath(rawPath string) string {
	return normalizeRenderedOperationPath(rawPath)
}
