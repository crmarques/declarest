package metadata

import (
	"fmt"
	"sort"
	"strings"
)

// ResourceDescription holds all gathered information about a resource or collection
// at a given logical path, suitable for presenting to the user.
type ResourceDescription struct {
	Path           string                 `json:"path" yaml:"path"`
	Collection     bool                   `json:"collection" yaml:"collection"`
	Identity       *IdentityDescription   `json:"identity,omitempty" yaml:"identity,omitempty"`
	PayloadType    string                 `json:"payloadType,omitempty" yaml:"payloadType,omitempty"`
	CollectionPath string                 `json:"collectionPath,omitempty" yaml:"collectionPath,omitempty"`
	Operations     []OperationDescription `json:"operations,omitempty" yaml:"operations,omitempty"`
	Schemas        []SchemaDescription    `json:"schemas,omitempty" yaml:"schemas,omitempty"`
	RequiredFields []string               `json:"requiredFields,omitempty" yaml:"requiredFields,omitempty"`
	SecretFields   []string               `json:"secretFields,omitempty" yaml:"secretFields,omitempty"`
}

// IdentityDescription describes how a resource is identified.
type IdentityDescription struct {
	ID    string `json:"id,omitempty" yaml:"id,omitempty"`
	Alias string `json:"alias,omitempty" yaml:"alias,omitempty"`
}

// OperationDescription describes a single operation available on the resource.
type OperationDescription struct {
	Name   string `json:"name" yaml:"name"`
	Method string `json:"method" yaml:"method"`
	Path   string `json:"path" yaml:"path"`
}

// SchemaDescription describes the payload schema for one operation.
type SchemaDescription struct {
	Operation  string       `json:"operation" yaml:"operation"`
	Method     string       `json:"method" yaml:"method"`
	Path       string       `json:"path" yaml:"path"`
	Source     string       `json:"source" yaml:"source"`
	Type       string       `json:"type" yaml:"type"`
	Properties []SchemaNode `json:"properties,omitempty" yaml:"properties,omitempty"`
	Items      *SchemaNode  `json:"items,omitempty" yaml:"items,omitempty"`
}

// SchemaNode describes a single property in a schema tree.
type SchemaNode struct {
	Name        string       `json:"name" yaml:"name"`
	Type        string       `json:"type" yaml:"type"`
	Required    bool         `json:"required,omitempty" yaml:"required,omitempty"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
	Format      string       `json:"format,omitempty" yaml:"format,omitempty"`
	Enum        []string     `json:"enum,omitempty" yaml:"enum,omitempty"`
	Default     any          `json:"default,omitempty" yaml:"default,omitempty"`
	Nullable    bool         `json:"nullable,omitempty" yaml:"nullable,omitempty"`
	MinLength   *int         `json:"minLength,omitempty" yaml:"minLength,omitempty"`
	MaxLength   *int         `json:"maxLength,omitempty" yaml:"maxLength,omitempty"`
	Minimum     *float64     `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Maximum     *float64     `json:"maximum,omitempty" yaml:"maximum,omitempty"`
	Pattern     string       `json:"pattern,omitempty" yaml:"pattern,omitempty"`
	Properties  []SchemaNode `json:"properties,omitempty" yaml:"properties,omitempty"`
	Items       *SchemaNode  `json:"items,omitempty" yaml:"items,omitempty"`
}

const describeMaxSchemaDepth = 6

// DescribeResource gathers all available information about a resource at the
// given logical path, using resolved metadata and an optional OpenAPI spec.
func DescribeResource(
	logicalPath string,
	md ResourceMetadata,
	openAPISpec any,
) ResourceDescription {
	descriptor, _ := ParsePathDescriptor(logicalPath)

	desc := ResourceDescription{
		Path:       logicalPath,
		Collection: descriptor.Collection,
	}

	if strings.TrimSpace(md.ID) != "" || strings.TrimSpace(md.Alias) != "" {
		desc.Identity = &IdentityDescription{
			ID:    md.ID,
			Alias: md.Alias,
		}
	}

	desc.PayloadType = md.PayloadType
	desc.CollectionPath = md.RemoteCollectionPath
	desc.RequiredFields = md.RequiredAttributes
	desc.SecretFields = md.SecretAttributes

	operationOrder := []string{
		string(OperationGet),
		string(OperationCreate),
		string(OperationUpdate),
		string(OperationDelete),
		string(OperationList),
		string(OperationCompare),
	}
	for _, opName := range operationOrder {
		spec, found := md.Operations[opName]
		if !found || strings.TrimSpace(spec.Path) == "" {
			continue
		}
		desc.Operations = append(desc.Operations, OperationDescription{
			Name:   opName,
			Method: spec.Method,
			Path:   spec.Path,
		})
	}

	desc.Schemas = describeOperationSchemas(md, openAPISpec)

	return desc
}

// describeOperationSchemas extracts schemas for write operations (create, update)
// from the OpenAPI spec. Falls back to get response schema if no write schemas found.
func describeOperationSchemas(md ResourceMetadata, openAPISpec any) []SchemaDescription {
	if openAPISpec == nil {
		return nil
	}

	pathItems := openAPIPathItems(openAPISpec)
	if len(pathItems) == 0 {
		return nil
	}

	var schemas []SchemaDescription

	// Try write operations first (most useful for the user)
	for _, opName := range []string{string(OperationCreate), string(OperationUpdate)} {
		spec, found := md.Operations[opName]
		if !found || strings.TrimSpace(spec.Method) == "" || strings.TrimSpace(spec.Path) == "" {
			continue
		}

		schema := describeRequestBodySchema(opName, spec, pathItems, openAPISpec)
		if schema != nil {
			schemas = append(schemas, *schema)
		}
	}

	if len(schemas) > 0 {
		return schemas
	}

	// Fallback to get response schema
	getSpec, found := md.Operations[string(OperationGet)]
	if !found || strings.TrimSpace(getSpec.Method) == "" || strings.TrimSpace(getSpec.Path) == "" {
		return nil
	}

	schema := describeResponseSchema(string(OperationGet), getSpec, pathItems, openAPISpec)
	if schema != nil {
		schemas = append(schemas, *schema)
	}

	return schemas
}

// describeRequestBodySchema finds and describes the request body schema for an operation.
func describeRequestBodySchema(
	opName string,
	spec OperationSpec,
	pathItems map[string]map[string]any,
	openAPISpec any,
) *SchemaDescription {
	pathItem, found := findPathItemForMetadataOperation(spec.Path, pathItems)
	if !found {
		return nil
	}

	method := strings.ToLower(strings.TrimSpace(spec.Method))
	operationItem, found := pathItem[method]
	if !found {
		return nil
	}
	operation, ok := asStringMap(operationItem)
	if !ok {
		return nil
	}

	requestBodySchema, _, found := inferOpenAPIRequestBodySchema(operation, openAPISpec)
	if !found {
		return nil
	}

	visited := make(map[string]struct{})
	resolved := resolveSchemaForDescribe(requestBodySchema, openAPISpec, visited, 0)
	if resolved == nil {
		return nil
	}

	rootType := describeSchemaTypeName(resolved, openAPISpec, make(map[string]struct{}), 0)
	requiredSet := collectAllRequiredFields(resolved, openAPISpec, make(map[string]struct{}), 0)
	properties := describeSchemaNodes(resolved, openAPISpec, requiredSet, make(map[string]struct{}), 0)
	if len(properties) == 0 {
		return nil
	}

	return &SchemaDescription{
		Operation:  opName,
		Method:     strings.ToUpper(spec.Method),
		Path:       spec.Path,
		Source:     "request-body",
		Type:       rootType,
		Properties: properties,
	}
}

// describeResponseSchema finds and describes the response schema for an operation.
func describeResponseSchema(
	opName string,
	spec OperationSpec,
	pathItems map[string]map[string]any,
	openAPISpec any,
) *SchemaDescription {
	pathItem, found := findPathItemForMetadataOperation(spec.Path, pathItems)
	if !found {
		return nil
	}

	method := strings.ToLower(strings.TrimSpace(spec.Method))
	operationItem, found := pathItem[method]
	if !found {
		return nil
	}
	operation, ok := asStringMap(operationItem)
	if !ok {
		return nil
	}

	responseSchema := findResponseSchema(operation, openAPISpec)
	if responseSchema == nil {
		return nil
	}

	visited := make(map[string]struct{})
	resolved := resolveSchemaForDescribe(responseSchema, openAPISpec, visited, 0)
	if resolved == nil {
		return nil
	}

	rootType := describeSchemaTypeName(resolved, openAPISpec, make(map[string]struct{}), 0)
	requiredSet := collectAllRequiredFields(resolved, openAPISpec, make(map[string]struct{}), 0)
	properties := describeSchemaNodes(resolved, openAPISpec, requiredSet, make(map[string]struct{}), 0)
	if len(properties) == 0 {
		return nil
	}

	return &SchemaDescription{
		Operation:  opName,
		Method:     strings.ToUpper(spec.Method),
		Path:       spec.Path,
		Source:     "response",
		Type:       rootType,
		Properties: properties,
	}
}

// findResponseSchema extracts the schema from the best-match response entry.
func findResponseSchema(operation map[string]any, openAPISpec any) any {
	responsesValue, found := operation["responses"]
	if !found {
		return nil
	}
	responses, ok := asStringMap(responsesValue)
	if !ok {
		return nil
	}

	for _, status := range []string{"200", "201", "202", "default"} {
		entry, found := responses[status]
		if !found {
			continue
		}
		schema := extractResponseEntrySchema(entry, openAPISpec)
		if schema != nil {
			return schema
		}
	}

	statuses := make([]string, 0, len(responses))
	for status := range responses {
		statuses = append(statuses, status)
	}
	sort.Strings(statuses)
	for _, status := range statuses {
		schema := extractResponseEntrySchema(responses[status], openAPISpec)
		if schema != nil {
			return schema
		}
	}

	return nil
}

func extractResponseEntrySchema(entry any, openAPISpec any) any {
	resolved, ok := resolveOpenAPIValueRefForInference(openAPISpec, entry, make(map[string]struct{}), 0)
	if !ok {
		return nil
	}
	response, ok := asStringMap(resolved)
	if !ok {
		return nil
	}
	contentValue, found := response["content"]
	if !found {
		return nil
	}
	content, ok := asStringMap(contentValue)
	if !ok {
		return nil
	}

	candidate, found := selectOpenAPIContentCandidate(content, openAPISpec)
	if !found || candidate.schema == nil {
		return nil
	}
	return candidate.schema
}

// findPathItemForMetadataOperation matches a metadata operation path template
// (e.g. /admin/realms/{{/realm}}/clients) against OpenAPI path items.
func findPathItemForMetadataOperation(
	metadataPath string,
	pathItems map[string]map[string]any,
) (map[string]any, bool) {
	opSegments := splitPathSegments(metadataPath)
	if len(opSegments) == 0 {
		return nil, false
	}

	for openAPIPath, pathItem := range pathItems {
		apiSegments := splitPathSegments(openAPIPath)
		if len(apiSegments) != len(opSegments) {
			continue
		}

		match := true
		for i := range opSegments {
			opSeg := opSegments[i]
			apiSeg := apiSegments[i]

			opIsTemplate := isMetadataTemplatePlaceholderSegment(opSeg)
			_, apiIsParam := openAPIPathParameterName(apiSeg)

			if opIsTemplate && apiIsParam {
				continue
			}
			if opSeg == apiSeg {
				continue
			}
			match = false
			break
		}

		if match {
			return pathItem, true
		}
	}

	return nil, false
}

// resolveSchemaForDescribe resolves $ref and returns a fully expanded schema map.
func resolveSchemaForDescribe(
	schema any,
	openAPISpec any,
	visited map[string]struct{},
	depth int,
) map[string]any {
	if depth > describeMaxSchemaDepth*4 {
		return nil
	}

	schemaMap, ok := asStringMap(schema)
	if !ok {
		return nil
	}

	if refValue, hasRef := schemaMap["$ref"]; hasRef {
		ref, ok := refValue.(string)
		if !ok || strings.TrimSpace(ref) == "" {
			return nil
		}
		ref = strings.TrimSpace(ref)
		if _, seen := visited[ref]; seen {
			return nil
		}

		resolved, found := resolveOpenAPIRef(openAPISpec, ref)
		if !found {
			return nil
		}

		visited[ref] = struct{}{}
		result := resolveSchemaForDescribe(resolved, openAPISpec, visited, depth+1)
		delete(visited, ref)
		return result
	}

	return schemaMap
}

// collectAllRequiredFields gathers required field names from a schema, including allOf.
func collectAllRequiredFields(
	schemaMap map[string]any,
	openAPISpec any,
	visited map[string]struct{},
	depth int,
) map[string]struct{} {
	if depth > describeMaxSchemaDepth*4 {
		return nil
	}

	required := make(map[string]struct{})

	if requiredValue, found := schemaMap["required"]; found {
		if names, ok := requiredValue.([]any); ok {
			for _, nameValue := range names {
				if name, ok := nameValue.(string); ok {
					trimmed := strings.TrimSpace(name)
					if trimmed != "" {
						required[trimmed] = struct{}{}
					}
				}
			}
		}
	}

	for _, combiner := range []string{"allOf", "anyOf"} {
		entries, ok := schemaMap[combiner]
		if !ok {
			continue
		}
		items, ok := entries.([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			resolved := resolveSchemaForDescribe(item, openAPISpec, cloneVisitedRefs(visited), depth+1)
			if resolved == nil {
				continue
			}
			for name := range collectAllRequiredFields(resolved, openAPISpec, cloneVisitedRefs(visited), depth+1) {
				required[name] = struct{}{}
			}
		}
	}

	return required
}

// describeSchemaNodes builds SchemaNode entries from a resolved schema.
func describeSchemaNodes(
	schemaMap map[string]any,
	openAPISpec any,
	requiredSet map[string]struct{},
	visited map[string]struct{},
	depth int,
) []SchemaNode {
	if depth > describeMaxSchemaDepth {
		return nil
	}

	allProperties := collectAllProperties(schemaMap, openAPISpec, cloneVisitedRefs(visited), depth)
	if len(allProperties) == 0 {
		return nil
	}

	names := make([]string, 0, len(allProperties))
	for name := range allProperties {
		names = append(names, name)
	}
	sort.Strings(names)

	nodes := make([]SchemaNode, 0, len(names))
	for _, name := range names {
		propSchema := allProperties[name]
		_, isRequired := requiredSet[name]
		node := describePropertyNode(name, propSchema, isRequired, openAPISpec, cloneVisitedRefs(visited), depth+1)
		nodes = append(nodes, node)
	}

	return nodes
}

// collectAllProperties gathers properties from a schema, merging allOf entries.
func collectAllProperties(
	schemaMap map[string]any,
	openAPISpec any,
	visited map[string]struct{},
	depth int,
) map[string]any {
	if depth > describeMaxSchemaDepth*4 {
		return nil
	}

	properties := make(map[string]any)

	if propsValue, found := schemaMap["properties"]; found {
		if props, ok := asStringMap(propsValue); ok {
			for name, propSchema := range props {
				trimmed := strings.TrimSpace(name)
				if trimmed != "" {
					properties[trimmed] = propSchema
				}
			}
		}
	}

	for _, combiner := range []string{"allOf", "anyOf", "oneOf"} {
		entries, ok := schemaMap[combiner]
		if !ok {
			continue
		}
		items, ok := entries.([]any)
		if !ok {
			continue
		}
		for _, item := range items {
			resolved := resolveSchemaForDescribe(item, openAPISpec, cloneVisitedRefs(visited), depth+1)
			if resolved == nil {
				continue
			}
			subProps := collectAllProperties(resolved, openAPISpec, cloneVisitedRefs(visited), depth+1)
			for name, schema := range subProps {
				if _, exists := properties[name]; !exists {
					properties[name] = schema
				}
			}
		}
	}

	return properties
}

// describePropertyNode builds a SchemaNode for a single property.
func describePropertyNode(
	name string,
	schema any,
	required bool,
	openAPISpec any,
	visited map[string]struct{},
	depth int,
) SchemaNode {
	resolved := resolveSchemaForDescribe(schema, openAPISpec, visited, depth)
	if resolved == nil {
		return SchemaNode{Name: name, Type: "any", Required: required}
	}

	node := SchemaNode{
		Name:        name,
		Required:    required,
		Description: asString(resolved["description"]),
		Format:      asString(resolved["format"]),
		Pattern:     asString(resolved["pattern"]),
	}

	if nullable, ok := resolved["nullable"].(bool); ok && nullable {
		node.Nullable = true
	}

	node.Default = resolved["default"]
	node.Enum = extractEnumStrings(resolved)
	node.MinLength = extractIntConstraint(resolved["minLength"])
	node.MaxLength = extractIntConstraint(resolved["maxLength"])
	node.Minimum = extractFloatConstraint(resolved["minimum"])
	node.Maximum = extractFloatConstraint(resolved["maximum"])

	node.Type = describeSchemaTypeName(resolved, openAPISpec, cloneVisitedRefs(visited), depth)

	if isDescribeObjectType(resolved) && depth < describeMaxSchemaDepth {
		childRequired := collectAllRequiredFields(resolved, openAPISpec, cloneVisitedRefs(visited), depth)
		node.Properties = describeSchemaNodes(resolved, openAPISpec, childRequired, cloneVisitedRefs(visited), depth)
	}

	if isDescribeArrayType(resolved) && depth < describeMaxSchemaDepth {
		if itemsValue, found := resolved["items"]; found {
			itemResolved := resolveSchemaForDescribe(itemsValue, openAPISpec, cloneVisitedRefs(visited), depth+1)
			if itemResolved != nil && isDescribeObjectType(itemResolved) {
				childRequired := collectAllRequiredFields(itemResolved, openAPISpec, cloneVisitedRefs(visited), depth+1)
				itemNode := SchemaNode{
					Name: "(items)",
					Type: describeSchemaTypeName(itemResolved, openAPISpec, cloneVisitedRefs(visited), depth+1),
				}
				itemNode.Properties = describeSchemaNodes(itemResolved, openAPISpec, childRequired, cloneVisitedRefs(visited), depth+1)
				node.Items = &itemNode
			}
		}
	}

	return node
}

// describeSchemaTypeName produces a human-friendly type name.
func describeSchemaTypeName(
	schemaMap map[string]any,
	openAPISpec any,
	visited map[string]struct{},
	depth int,
) string {
	typeName := asString(schemaMap["type"])

	if typeName == "array" {
		if itemsValue, found := schemaMap["items"]; found {
			itemResolved := resolveSchemaForDescribe(itemsValue, openAPISpec, visited, depth+1)
			if itemResolved != nil {
				itemType := describeSchemaTypeName(itemResolved, openAPISpec, cloneVisitedRefs(visited), depth+1)
				return itemType + "[]"
			}
		}
		return "array"
	}

	if typeName == "object" || typeName == "" {
		_, hasProperties := schemaMap["properties"]
		if !hasProperties {
			if addlValue, hasAddl := schemaMap["additionalProperties"]; hasAddl {
				if addlMap, ok := asStringMap(addlValue); ok {
					addlResolved := resolveSchemaForDescribe(addlMap, openAPISpec, cloneVisitedRefs(visited), depth+1)
					if addlResolved != nil {
						valueType := describeSchemaTypeName(addlResolved, openAPISpec, cloneVisitedRefs(visited), depth+1)
						return "map[string]" + valueType
					}
				}
				return "map[string]any"
			}
		}

		hasCombiner := false
		for _, key := range []string{"allOf", "oneOf", "anyOf"} {
			if _, found := schemaMap[key]; found {
				hasCombiner = true
				break
			}
		}

		if hasProperties || hasCombiner {
			return "object"
		}

		if typeName == "object" {
			return "object"
		}

		return "any"
	}

	return typeName
}

func isDescribeObjectType(schemaMap map[string]any) bool {
	typeName := asString(schemaMap["type"])
	if typeName == "object" {
		return true
	}
	if _, hasProps := schemaMap["properties"]; hasProps {
		return true
	}
	for _, key := range []string{"allOf", "oneOf", "anyOf"} {
		if _, found := schemaMap[key]; found {
			return true
		}
	}
	return false
}

func isDescribeArrayType(schemaMap map[string]any) bool {
	return asString(schemaMap["type"]) == "array"
}

func extractEnumStrings(schemaMap map[string]any) []string {
	rawEnum, found := schemaMap["enum"]
	if !found {
		return nil
	}
	items, ok := rawEnum.([]any)
	if !ok || len(items) == 0 {
		return nil
	}

	values := make([]string, 0, len(items))
	for _, item := range items {
		values = append(values, fmt.Sprint(item))
	}
	return values
}

func extractIntConstraint(value any) *int {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		v := int(typed)
		return &v
	case int:
		return &typed
	default:
		return nil
	}
}

func extractFloatConstraint(value any) *float64 {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case float64:
		return &typed
	case int:
		v := float64(typed)
		return &v
	default:
		return nil
	}
}

func cloneVisitedRefs(visited map[string]struct{}) map[string]struct{} {
	if len(visited) == 0 {
		return make(map[string]struct{})
	}
	clone := make(map[string]struct{}, len(visited))
	for key := range visited {
		clone[key] = struct{}{}
	}
	return clone
}
