package http

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/templatescope"
	"github.com/crmarques/declarest/resource"
)

func (g *HTTPResourceServerGateway) validateOperationPayload(
	ctx context.Context,
	operation metadata.Operation,
	resourceInfo resource.Resource,
	spec metadata.OperationSpec,
) error {
	if spec.Validate == nil {
		return nil
	}

	normalizedBody, err := resource.Normalize(spec.Body)
	if err != nil {
		return err
	}
	derivedFields := deriveValidationPathFields(resourceInfo)

	payloadView := normalizedBody
	if len(derivedFields) > 0 {
		payloadView = mergeValidationPayloadFields(normalizedBody, derivedFields)
	}

	if err := validateOperationRequiredAttributes(payloadView, spec.Validate.RequiredAttributes); err != nil {
		return err
	}
	if err := g.validateOperationAssertions(ctx, payloadView, spec.Validate.Assertions); err != nil {
		return err
	}
	if err := g.validateOperationSchemaRef(
		ctx,
		normalizedBody,
		derivedFields,
		spec.Path,
		spec.Method,
		spec.Validate.SchemaRef,
	); err != nil {
		return err
	}

	return nil
}

func mergeValidationPayloadFields(normalizedBody resource.Value, derivedFields map[string]any) resource.Value {
	if len(derivedFields) == 0 {
		return normalizedBody
	}
	baseObject, isObject := normalizedBody.(map[string]any)
	if !isObject {
		baseObject = map[string]any{}
		if normalizedBody != nil {
			baseObject["payload"] = normalizedBody
		}
	}

	merged := make(map[string]any, len(baseObject)+len(derivedFields))
	for key, value := range baseObject {
		merged[key] = value
	}
	keys := sortedMapKeysAny(derivedFields)
	for _, key := range keys {
		if _, exists := merged[key]; exists {
			continue
		}
		merged[key] = derivedFields[key]
	}

	return merged
}

func deriveValidationPathFields(resourceInfo resource.Resource) map[string]any {
	fields := map[string]any{}

	for key, value := range templatescope.DerivePathTemplateFields(resourceInfo.LogicalPath, resourceInfo.Metadata) {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		fields[trimmedKey] = trimmedValue
	}

	if aliasAttribute := strings.TrimSpace(resourceInfo.Metadata.AliasFromAttribute); aliasAttribute != "" {
		if strings.TrimSpace(resourceInfo.LocalAlias) != "" {
			if _, exists := fields[aliasAttribute]; !exists {
				fields[aliasAttribute] = strings.TrimSpace(resourceInfo.LocalAlias)
			}
		}
	}
	if idAttribute := strings.TrimSpace(resourceInfo.Metadata.IDFromAttribute); idAttribute != "" {
		if strings.TrimSpace(resourceInfo.RemoteID) != "" {
			if _, exists := fields[idAttribute]; !exists {
				fields[idAttribute] = strings.TrimSpace(resourceInfo.RemoteID)
			}
		}
	}

	if len(fields) == 0 {
		return nil
	}
	return fields
}

func validateOperationRequiredAttributes(payload resource.Value, attributes []string) error {
	names, err := normalizePayloadAttributeNames("validate.requiredAttributes", attributes)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}

	objectPayload, ok := payload.(map[string]any)
	if !ok {
		return validationError(
			"operation payload validation requiredAttributes requires an object payload",
			nil,
		)
	}

	missing := make([]string, 0)
	for _, name := range names {
		value, exists := objectPayload[name]
		if !exists || value == nil {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return validationError(
			fmt.Sprintf(
				"operation payload validation failed: missing required attributes [%s]",
				strings.Join(missing, ", "),
			),
			nil,
		)
	}

	return nil
}

func (g *HTTPResourceServerGateway) validateOperationAssertions(
	ctx context.Context,
	payload resource.Value,
	assertions []metadata.ValidationAssertion,
) error {
	if len(assertions) == 0 {
		return nil
	}

	for idx, assertion := range assertions {
		expression := strings.TrimSpace(assertion.JQ)
		if expression == "" {
			continue
		}

		code, err := g.compileListJQCode(ctx, expression)
		if err != nil {
			return validationError(
				fmt.Sprintf("invalid payload validation assertion[%d] jq expression", idx),
				err,
			)
		}

		runCtx := ctx
		if runCtx == nil {
			runCtx = context.Background()
		}

		iterator := code.RunWithContext(runCtx, payload)
		satisfied, evalErr := evaluateAssertionResults(iterator)
		if evalErr != nil {
			return validationError(
				fmt.Sprintf("failed to evaluate payload validation assertion[%d]", idx),
				evalErr,
			)
		}
		if satisfied {
			continue
		}

		message := strings.TrimSpace(assertion.Message)
		if message == "" {
			message = fmt.Sprintf("payload validation assertion[%d] failed", idx)
		}
		return validationError(message, nil)
	}

	return nil
}

func evaluateAssertionResults(iterator anyIterator) (bool, error) {
	hasResult := false
	satisfied := false

	for {
		value, ok := iterator.Next()
		if !ok {
			break
		}
		if valueErr, isErr := value.(error); isErr {
			return false, valueErr
		}
		hasResult = true
		if jqValueTruthy(value) {
			satisfied = true
		}
	}

	if !hasResult {
		return false, nil
	}
	return satisfied, nil
}

type anyIterator interface {
	Next() (any, bool)
}

func jqValueTruthy(value any) bool {
	if value == nil {
		return false
	}
	if typed, ok := value.(bool); ok {
		return typed
	}
	return true
}

func (g *HTTPResourceServerGateway) validateOperationSchemaRef(
	ctx context.Context,
	payload resource.Value,
	derivedFields map[string]any,
	requestPath string,
	requestMethod string,
	schemaRef string,
) error {
	trimmedSchemaRef := strings.TrimSpace(schemaRef)
	if trimmedSchemaRef == "" {
		return nil
	}

	schema, document, err := g.resolveOpenAPISchemaForValidation(ctx, requestPath, requestMethod, trimmedSchemaRef)
	if err != nil {
		return err
	}

	schemaPayload := payload
	if len(derivedFields) > 0 {
		schemaPayload = augmentSchemaValidationPayload(payload, derivedFields, schema, document)
	}

	if err := validateValueAgainstOpenAPISchema(schemaPayload, schema, document, "$", map[string]struct{}{}, 0); err != nil {
		return validationError(
			fmt.Sprintf(
				"operation payload validation failed for schemaRef %q: %v",
				trimmedSchemaRef,
				err,
			),
			nil,
		)
	}
	return nil
}

func augmentSchemaValidationPayload(
	payload resource.Value,
	derivedFields map[string]any,
	schema any,
	document map[string]any,
) resource.Value {
	payloadObject, ok := payload.(map[string]any)
	if !ok {
		return payload
	}

	allowedFields := topLevelSchemaObjectFieldNames(schema, document, map[string]struct{}{}, 0)
	if len(allowedFields) == 0 {
		return payload
	}

	merged := make(map[string]any, len(payloadObject)+len(derivedFields))
	for key, value := range payloadObject {
		merged[key] = value
	}
	for _, key := range sortedMapKeysAny(derivedFields) {
		if _, allowed := allowedFields[key]; !allowed {
			continue
		}
		if _, exists := merged[key]; exists {
			continue
		}
		merged[key] = derivedFields[key]
	}

	return merged
}

func (g *HTTPResourceServerGateway) resolveOpenAPISchemaForValidation(
	ctx context.Context,
	requestPath string,
	requestMethod string,
	schemaRef string,
) (any, map[string]any, error) {
	if strings.TrimSpace(g.openAPISource) == "" {
		return nil, nil, validationError(
			"validate.schemaRef requires resource-server.http.openapi to be configured",
			nil,
		)
	}

	document, err := g.openAPIDocument(ctx)
	if err != nil {
		return nil, nil, err
	}

	if schemaRef == "openapi:request-body" {
		_, pathItem, found := findOpenAPIPathItem(document, requestPath)
		if !found {
			return nil, nil, validationError(
				fmt.Sprintf("OpenAPI path %q was not found for validate.schemaRef", requestPath),
				nil,
			)
		}

		method := strings.ToUpper(strings.TrimSpace(requestMethod))
		if method == "" {
			return nil, nil, validationError("request method is required for OpenAPI request-body validation", nil)
		}
		operationItem, found := openAPIPathMethod(pathItem, method)
		if !found {
			return nil, nil, validationError(
				fmt.Sprintf(
					"OpenAPI path %q does not support method %s for validate.schemaRef",
					requestPath,
					method,
				),
				nil,
			)
		}

		schema, found := openAPIRequestBodySchemaForValidation(document, operationItem)
		if !found {
			return nil, nil, validationError(
				fmt.Sprintf(
					"OpenAPI request body schema was not found for %s %q",
					method,
					requestPath,
				),
				nil,
			)
		}
		return schema, document, nil
	}

	if strings.HasPrefix(schemaRef, "openapi:#/") {
		ref := strings.TrimPrefix(schemaRef, "openapi:")
		resolved, found := resolveOpenAPIJSONPointer(document, ref)
		if !found {
			return nil, nil, validationError(
				fmt.Sprintf("OpenAPI schema reference %q could not be resolved", schemaRef),
				nil,
			)
		}
		return resolved, document, nil
	}

	return nil, nil, validationError(
		fmt.Sprintf("validate.schemaRef %q is not supported", schemaRef),
		nil,
	)
}

func openAPIRequestBodySchemaForValidation(document map[string]any, operation map[string]any) (any, bool) {
	requestBodyValue, found := operation["requestBody"]
	if !found {
		return nil, false
	}

	resolvedRequestBody, ok := resolveOpenAPIValueRef(document, requestBodyValue, map[string]struct{}{}, 0)
	if !ok {
		return nil, false
	}

	requestBody, ok := asStringAnyMap(resolvedRequestBody)
	if !ok {
		return nil, false
	}

	contentValue, found := requestBody["content"]
	if !found {
		return nil, false
	}
	content, ok := asStringAnyMap(contentValue)
	if !ok || len(content) == 0 {
		return nil, false
	}

	mediaTypes := make([]string, 0, len(content))
	for mediaType := range content {
		mediaTypes = append(mediaTypes, mediaType)
	}
	sort.Slice(mediaTypes, func(i int, j int) bool {
		leftScore := openAPIMediaTypePriority(mediaTypes[i])
		rightScore := openAPIMediaTypePriority(mediaTypes[j])
		if leftScore != rightScore {
			return leftScore < rightScore
		}
		return mediaTypes[i] < mediaTypes[j]
	})

	for _, mediaType := range mediaTypes {
		mediaValue, ok := asStringAnyMap(content[mediaType])
		if !ok {
			continue
		}
		schemaValue, hasSchema := mediaValue["schema"]
		if !hasSchema {
			continue
		}
		return schemaValue, true
	}

	return nil, false
}

func openAPIMediaTypePriority(value string) int {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch {
	case normalized == "application/json":
		return 0
	case strings.HasPrefix(normalized, "application/") && strings.HasSuffix(normalized, "+json"):
		return 1
	default:
		return 2
	}
}

func resolveOpenAPIValueRef(
	document map[string]any,
	value any,
	visited map[string]struct{},
	depth int,
) (any, bool) {
	if depth > 64 {
		return nil, false
	}

	mapped, ok := asStringAnyMap(value)
	if !ok {
		return value, true
	}

	refValue, hasRef := mapped["$ref"]
	if !hasRef {
		return mapped, true
	}

	ref, ok := refValue.(string)
	if !ok || strings.TrimSpace(ref) == "" {
		return nil, false
	}
	ref = strings.TrimSpace(ref)
	if _, exists := visited[ref]; exists {
		return nil, false
	}

	resolved, found := resolveOpenAPIJSONPointer(document, ref)
	if !found {
		return nil, false
	}

	visited[ref] = struct{}{}
	finalValue, ok := resolveOpenAPIValueRef(document, resolved, visited, depth+1)
	delete(visited, ref)
	return finalValue, ok
}

func resolveOpenAPIJSONPointer(document map[string]any, ref string) (any, bool) {
	trimmedRef := strings.TrimSpace(ref)
	if !strings.HasPrefix(trimmedRef, "#/") {
		return nil, false
	}

	pointer := strings.TrimPrefix(trimmedRef, "#/")
	if strings.TrimSpace(pointer) == "" {
		return document, true
	}

	current := any(document)
	segments := strings.Split(pointer, "/")
	for _, rawSegment := range segments {
		segment := strings.ReplaceAll(strings.ReplaceAll(rawSegment, "~1", "/"), "~0", "~")
		currentMap, ok := asStringAnyMap(current)
		if !ok {
			return nil, false
		}
		next, found := currentMap[segment]
		if !found {
			return nil, false
		}
		current = next
	}
	return current, true
}

func validateValueAgainstOpenAPISchema(
	value any,
	schema any,
	document map[string]any,
	location string,
	visitedRefs map[string]struct{},
	depth int,
) error {
	if depth > 96 {
		return fmt.Errorf("%s schema nesting exceeds supported depth", location)
	}

	resolvedSchema, err := resolveSchemaValue(document, schema, visitedRefs, depth)
	if err != nil {
		return fmt.Errorf("%s %w", location, err)
	}
	if len(resolvedSchema) == 0 {
		return nil
	}

	if nullableSchema(resolvedSchema) && value == nil {
		return nil
	}

	if err := validateSchemaCombiners(value, resolvedSchema, document, location, depth); err != nil {
		return err
	}
	if err := validateSchemaEnum(value, resolvedSchema, location); err != nil {
		return err
	}
	if err := validateSchemaType(value, resolvedSchema, location); err != nil {
		return err
	}
	if err := validateSchemaObject(value, resolvedSchema, document, location, depth); err != nil {
		return err
	}
	if err := validateSchemaArray(value, resolvedSchema, document, location, depth); err != nil {
		return err
	}

	return nil
}

func resolveSchemaValue(
	document map[string]any,
	schema any,
	visitedRefs map[string]struct{},
	depth int,
) (map[string]any, error) {
	if depth > 96 {
		return nil, fmt.Errorf("schema reference depth exceeded")
	}

	schemaMap, ok := asStringAnyMap(schema)
	if !ok {
		return nil, nil
	}

	refValue, hasRef := schemaMap["$ref"]
	if !hasRef {
		return schemaMap, nil
	}

	ref, ok := refValue.(string)
	if !ok || strings.TrimSpace(ref) == "" {
		return nil, fmt.Errorf("schema $ref must be a non-empty string")
	}
	ref = strings.TrimSpace(ref)
	if _, exists := visitedRefs[ref]; exists {
		return nil, fmt.Errorf("schema reference cycle detected for %q", ref)
	}

	resolved, found := resolveOpenAPIJSONPointer(document, ref)
	if !found {
		return nil, fmt.Errorf("schema reference %q was not found", ref)
	}

	visitedRefs[ref] = struct{}{}
	resolvedSchema, err := resolveSchemaValue(document, resolved, visitedRefs, depth+1)
	delete(visitedRefs, ref)
	if err != nil {
		return nil, err
	}
	return resolvedSchema, nil
}

func topLevelSchemaObjectFieldNames(
	schema any,
	document map[string]any,
	visitedRefs map[string]struct{},
	depth int,
) map[string]struct{} {
	if depth > 24 {
		return nil
	}

	resolvedSchema, err := resolveSchemaValue(document, schema, visitedRefs, depth)
	if err != nil || len(resolvedSchema) == 0 {
		return nil
	}

	fields := map[string]struct{}{}
	for _, name := range requiredPropertyNames(resolvedSchema["required"]) {
		fields[name] = struct{}{}
	}
	if properties, ok := asStringAnyMap(resolvedSchema["properties"]); ok {
		for name := range properties {
			trimmed := strings.TrimSpace(name)
			if trimmed == "" {
				continue
			}
			fields[trimmed] = struct{}{}
		}
	}

	if allOf, ok := schemaSlice(resolvedSchema["allOf"]); ok {
		for _, item := range allOf {
			merged := topLevelSchemaObjectFieldNames(item, document, map[string]struct{}{}, depth+1)
			for key := range merged {
				fields[key] = struct{}{}
			}
		}
	}

	if len(fields) == 0 {
		return nil
	}
	return fields
}

func nullableSchema(schema map[string]any) bool {
	nullable, ok := schema["nullable"].(bool)
	return ok && nullable
}

func validateSchemaCombiners(
	value any,
	schema map[string]any,
	document map[string]any,
	location string,
	depth int,
) error {
	if allOf, ok := schemaSlice(schema["allOf"]); ok && len(allOf) > 0 {
		for idx, item := range allOf {
			if err := validateValueAgainstOpenAPISchema(
				value,
				item,
				document,
				fmt.Sprintf("%s allOf[%d]", location, idx),
				map[string]struct{}{},
				depth+1,
			); err != nil {
				return err
			}
		}
	}

	if anyOf, ok := schemaSlice(schema["anyOf"]); ok && len(anyOf) > 0 {
		anyMatched := false
		var firstErr error
		for idx, item := range anyOf {
			err := validateValueAgainstOpenAPISchema(
				value,
				item,
				document,
				fmt.Sprintf("%s anyOf[%d]", location, idx),
				map[string]struct{}{},
				depth+1,
			)
			if err == nil {
				anyMatched = true
				break
			}
			if firstErr == nil {
				firstErr = err
			}
		}
		if !anyMatched {
			if firstErr != nil {
				return firstErr
			}
			return fmt.Errorf("%s did not match any schema in anyOf", location)
		}
	}

	if oneOf, ok := schemaSlice(schema["oneOf"]); ok && len(oneOf) > 0 {
		matches := 0
		for idx, item := range oneOf {
			err := validateValueAgainstOpenAPISchema(
				value,
				item,
				document,
				fmt.Sprintf("%s oneOf[%d]", location, idx),
				map[string]struct{}{},
				depth+1,
			)
			if err == nil {
				matches++
			}
		}
		if matches != 1 {
			return fmt.Errorf("%s expected exactly one oneOf schema match, got %d", location, matches)
		}
	}

	return nil
}

func validateSchemaEnum(value any, schema map[string]any, location string) error {
	enumValues, ok := schemaSlice(schema["enum"])
	if !ok || len(enumValues) == 0 {
		return nil
	}

	for _, candidate := range enumValues {
		if reflect.DeepEqual(candidate, value) {
			return nil
		}
	}
	return fmt.Errorf("%s value is not allowed by enum", location)
}

func validateSchemaType(value any, schema map[string]any, location string) error {
	types := schemaTypeNames(schema["type"])
	if len(types) == 0 {
		return nil
	}

	for _, schemaType := range types {
		if valueMatchesSchemaType(value, schemaType) {
			return nil
		}
	}

	return fmt.Errorf(
		"%s expected type [%s], got %s",
		location,
		strings.Join(types, ", "),
		describeValueType(value),
	)
}

func validateSchemaObject(
	value any,
	schema map[string]any,
	document map[string]any,
	location string,
	depth int,
) error {
	properties, hasProperties := asStringAnyMap(schema["properties"])
	required := requiredPropertyNames(schema["required"])
	_, hasAdditional := schema["additionalProperties"]
	expectsObject := hasProperties || len(required) > 0 || hasAdditional || schemaTypeContains(schema["type"], "object")
	if !expectsObject {
		return nil
	}

	objectValue, ok := value.(map[string]any)
	if !ok {
		return fmt.Errorf("%s expected object, got %s", location, describeValueType(value))
	}

	for _, propertyName := range required {
		if _, exists := objectValue[propertyName]; !exists {
			return fmt.Errorf("%s missing required property %q", location, propertyName)
		}
	}

	for propertyName, propertySchema := range properties {
		propertyValue, exists := objectValue[propertyName]
		if !exists {
			continue
		}
		if err := validateValueAgainstOpenAPISchema(
			propertyValue,
			propertySchema,
			document,
			dotPath(location, propertyName),
			map[string]struct{}{},
			depth+1,
		); err != nil {
			return err
		}
	}

	if additionalValue, hasAdditional := schema["additionalProperties"]; hasAdditional {
		switch typed := additionalValue.(type) {
		case bool:
			if !typed {
				for key := range objectValue {
					if _, known := properties[key]; known {
						continue
					}
					return fmt.Errorf("%s property %q is not allowed", location, key)
				}
			}
		default:
			for key, propertyValue := range objectValue {
				if _, known := properties[key]; known {
					continue
				}
				if err := validateValueAgainstOpenAPISchema(
					propertyValue,
					typed,
					document,
					dotPath(location, key),
					map[string]struct{}{},
					depth+1,
				); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func validateSchemaArray(
	value any,
	schema map[string]any,
	document map[string]any,
	location string,
	depth int,
) error {
	itemsValue, hasItems := schema["items"]
	expectsArray := hasItems || schemaTypeContains(schema["type"], "array")
	if !expectsArray {
		return nil
	}

	arrayValue, ok := value.([]any)
	if !ok {
		return fmt.Errorf("%s expected array, got %s", location, describeValueType(value))
	}

	if !hasItems {
		return nil
	}

	if tupleItems, ok := schemaSlice(itemsValue); ok {
		for idx := 0; idx < len(tupleItems) && idx < len(arrayValue); idx++ {
			if err := validateValueAgainstOpenAPISchema(
				arrayValue[idx],
				tupleItems[idx],
				document,
				fmt.Sprintf("%s[%d]", location, idx),
				map[string]struct{}{},
				depth+1,
			); err != nil {
				return err
			}
		}
		return nil
	}

	for idx, item := range arrayValue {
		if err := validateValueAgainstOpenAPISchema(
			item,
			itemsValue,
			document,
			fmt.Sprintf("%s[%d]", location, idx),
			map[string]struct{}{},
			depth+1,
		); err != nil {
			return err
		}
	}

	return nil
}

func requiredPropertyNames(value any) []string {
	rawValues, ok := schemaSlice(value)
	if !ok || len(rawValues) == 0 {
		return nil
	}

	names := make([]string, 0, len(rawValues))
	seen := make(map[string]struct{}, len(rawValues))
	for _, raw := range rawValues {
		name, ok := raw.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		names = append(names, trimmed)
	}
	return names
}

func schemaTypeNames(value any) []string {
	if value == nil {
		return nil
	}
	if single, ok := value.(string); ok {
		trimmed := strings.ToLower(strings.TrimSpace(single))
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	}

	rawValues, ok := schemaSlice(value)
	if !ok {
		return nil
	}

	types := make([]string, 0, len(rawValues))
	seen := make(map[string]struct{}, len(rawValues))
	for _, raw := range rawValues {
		name, ok := raw.(string)
		if !ok {
			continue
		}
		trimmed := strings.ToLower(strings.TrimSpace(name))
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		types = append(types, trimmed)
	}
	return types
}

func schemaTypeContains(value any, expected string) bool {
	expectedNormalized := strings.ToLower(strings.TrimSpace(expected))
	if expectedNormalized == "" {
		return false
	}
	for _, actual := range schemaTypeNames(value) {
		if actual == expectedNormalized {
			return true
		}
	}
	return false
}

func valueMatchesSchemaType(value any, schemaType string) bool {
	switch strings.ToLower(strings.TrimSpace(schemaType)) {
	case "null":
		return value == nil
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "integer":
		switch typed := value.(type) {
		case int:
			return true
		case int8:
			return true
		case int16:
			return true
		case int32:
			return true
		case int64:
			return true
		case uint:
			return true
		case uint8:
			return true
		case uint16:
			return true
		case uint32:
			return true
		case uint64:
			return true
		case float64:
			return math.Trunc(typed) == typed
		case float32:
			return math.Trunc(float64(typed)) == float64(typed)
		default:
			return false
		}
	case "number":
		switch value.(type) {
		case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func describeValueType(value any) string {
	if value == nil {
		return "null"
	}
	switch value.(type) {
	case map[string]any:
		return "object"
	case []any:
		return "array"
	case string:
		return "string"
	case bool:
		return "boolean"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "integer"
	case float32, float64:
		return "number"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func schemaSlice(value any) ([]any, bool) {
	switch typed := value.(type) {
	case []any:
		return typed, true
	default:
		return nil, false
	}
}

func asStringAnyMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		mapped := make(map[string]any, len(typed))
		for key, item := range typed {
			keyText, ok := key.(string)
			if !ok {
				return nil, false
			}
			mapped[keyText] = item
		}
		return mapped, true
	default:
		return nil, false
	}
}

func dotPath(base string, field string) string {
	trimmedField := strings.TrimSpace(field)
	if trimmedField == "" {
		return base
	}
	if base == "" || base == "$" {
		return "$." + trimmedField
	}
	return base + "." + trimmedField
}

func sortedMapKeysAny(values map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
