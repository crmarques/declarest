package metadata

import (
	"bytes"
	"encoding/json"
	"strings"

	"go.yaml.in/yaml/v3"
)

type resourceMetadataWire struct {
	ResourceInfo  *resourceInfoWire  `json:"resourceInfo,omitempty" yaml:"resourceInfo,omitempty"`
	OperationInfo *operationInfoWire `json:"operationInfo,omitempty" yaml:"operationInfo,omitempty"`
}

type resourceInfoWire struct {
	IDFromAttribute        string                       `json:"idFromAttribute,omitempty" yaml:"idFromAttribute,omitempty"`
	AliasFromAttribute     string                       `json:"aliasFromAttribute,omitempty" yaml:"aliasFromAttribute,omitempty"`
	CollectionPath         string                       `json:"collectionPath,omitempty" yaml:"collectionPath,omitempty"`
	PayloadType            string                       `json:"payloadType,omitempty" yaml:"payloadType,omitempty"`
	SecretInAttributes     *[]string                    `json:"secretInAttributes,omitempty" yaml:"secretInAttributes,omitempty"`
	ExternalizedAttributes *[]externalizedAttributeWire `json:"externalizedAttributes,omitempty" yaml:"externalizedAttributes,omitempty"`
}

type externalizedAttributeWire struct {
	Path           *[]string `json:"path,omitempty" yaml:"path,omitempty"`
	File           string    `json:"file,omitempty" yaml:"file,omitempty"`
	Template       string    `json:"template,omitempty" yaml:"template,omitempty"`
	Mode           string    `json:"mode,omitempty" yaml:"mode,omitempty"`
	SaveBehavior   string    `json:"saveBehavior,omitempty" yaml:"saveBehavior,omitempty"`
	RenderBehavior string    `json:"renderBehavior,omitempty" yaml:"renderBehavior,omitempty"`
	Enabled        *bool     `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type operationInfoWire struct {
	Defaults         *operationDefaultsWire `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	GetResource      *resourceOperationWire `json:"getResource,omitempty" yaml:"getResource,omitempty"`
	CreateResource   *resourceOperationWire `json:"createResource,omitempty" yaml:"createResource,omitempty"`
	UpdateResource   *resourceOperationWire `json:"updateResource,omitempty" yaml:"updateResource,omitempty"`
	DeleteResource   *resourceOperationWire `json:"deleteResource,omitempty" yaml:"deleteResource,omitempty"`
	ListCollection   *resourceOperationWire `json:"listCollection,omitempty" yaml:"listCollection,omitempty"`
	CompareResources *resourceOperationWire `json:"compareResources,omitempty" yaml:"compareResources,omitempty"`
}

type operationDefaultsWire struct {
	Payload *payloadTransformWire `json:"payload,omitempty" yaml:"payload,omitempty"`
}

type payloadTransformWire struct {
	FilterAttributes   *stringListWire `json:"filterAttributes,omitempty" yaml:"filterAttributes,omitempty"`
	SuppressAttributes *stringListWire `json:"suppressAttributes,omitempty" yaml:"suppressAttributes,omitempty"`
	JQExpression       *string         `json:"jqExpression,omitempty" yaml:"jqExpression,omitempty"`

	Order []string `json:"-" yaml:"-"`
}

type validationAssertionWire struct {
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	JQ      string `json:"jq,omitempty" yaml:"jq,omitempty"`
}

type operationValidationWire struct {
	RequiredAttributes *stringListWire            `json:"requiredAttributes,omitempty" yaml:"requiredAttributes,omitempty"`
	Assertions         *[]validationAssertionWire `json:"assertions,omitempty" yaml:"assertions,omitempty"`
	SchemaRef          string                     `json:"schemaRef,omitempty" yaml:"schemaRef,omitempty"`
}

type httpHeaderWire struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

type resourceOperationWire struct {
	HTTPMethod  string                   `json:"httpMethod,omitempty" yaml:"httpMethod,omitempty"`
	Path        string                   `json:"path,omitempty" yaml:"path,omitempty"`
	Query       *map[string]string       `json:"query,omitempty" yaml:"query,omitempty"`
	HTTPHeaders *[]httpHeaderWire        `json:"httpHeaders,omitempty" yaml:"httpHeaders,omitempty"`
	Body        any                      `json:"body,omitempty" yaml:"body,omitempty"`
	Payload     *payloadTransformWire    `json:"payload,omitempty" yaml:"payload,omitempty"`
	Validate    *operationValidationWire `json:"validate,omitempty" yaml:"validate,omitempty"`

	FilterAttributes   *stringListWire `json:"filterAttributes,omitempty" yaml:"filterAttributes,omitempty"`
	SuppressAttributes *stringListWire `json:"suppressAttributes,omitempty" yaml:"suppressAttributes,omitempty"`
	JQExpression       *string         `json:"jqExpression,omitempty" yaml:"jqExpression,omitempty"`
}

func (m ResourceMetadata) MarshalJSON() ([]byte, error) {
	return EncodeResourceMetadataJSON(m, false)
}

func (m *ResourceMetadata) UnmarshalJSON(data []byte) error {
	decoded, err := DecodeResourceMetadataJSON(data)
	if err != nil {
		return err
	}
	*m = decoded
	return nil
}

func (m ResourceMetadata) MarshalYAML() (any, error) {
	return resourceMetadataToWire(m), nil
}

func (p *payloadTransformWire) UnmarshalJSON(data []byte) error {
	type alias payloadTransformWire
	decoded := alias{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	decoded.Order = payloadTransformOrderFromJSON(data)
	*p = payloadTransformWire(decoded)
	return nil
}

func (p *payloadTransformWire) UnmarshalYAML(value *yaml.Node) error {
	type alias payloadTransformWire
	decoded := alias{}
	if err := value.Decode(&decoded); err != nil {
		return err
	}
	decoded.Order = payloadTransformOrderFromYAMLNode(value)
	*p = payloadTransformWire(decoded)
	return nil
}

func (m *ResourceMetadata) UnmarshalYAML(value *yaml.Node) error {
	wire := resourceMetadataWire{}
	if err := value.Decode(&wire); err != nil {
		return err
	}

	*m = resourceMetadataFromWire(wire)
	return nil
}

func DecodeResourceMetadataJSON(data []byte) (ResourceMetadata, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	wire := resourceMetadataWire{}
	if err := decoder.Decode(&wire); err != nil {
		return ResourceMetadata{}, err
	}

	return resourceMetadataFromWire(wire), nil
}

func DecodeResourceMetadataYAML(data []byte) (ResourceMetadata, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	wire := resourceMetadataWire{}
	if err := decoder.Decode(&wire); err != nil {
		return ResourceMetadata{}, err
	}

	return resourceMetadataFromWire(wire), nil
}

func EncodeResourceMetadataJSON(metadata ResourceMetadata, pretty bool) ([]byte, error) {
	wire := resourceMetadataToWire(metadata)
	if pretty {
		return json.MarshalIndent(wire, "", "  ")
	}
	return json.Marshal(wire)
}

func EncodeResourceMetadataYAML(metadata ResourceMetadata) ([]byte, error) {
	return yaml.Marshal(resourceMetadataToWire(metadata))
}

func EffectiveCollectionPath(md ResourceMetadata, fallback string) string {
	if override := strings.TrimSpace(md.CollectionPath); override != "" {
		return override
	}
	return fallback
}

func resourceMetadataToWire(metadata ResourceMetadata) resourceMetadataWire {
	wire := resourceMetadataWire{}

	resourceInfo := resourceInfoWire{
		IDFromAttribute:    metadata.IDFromAttribute,
		AliasFromAttribute: metadata.AliasFromAttribute,
		CollectionPath:     metadata.CollectionPath,
		PayloadType:        metadata.PayloadType,
	}
	if metadata.SecretsFromAttributes != nil {
		resourceInfo.SecretInAttributes = stringSlicePointer(metadata.SecretsFromAttributes)
	}
	if metadata.ExternalizedAttributes != nil {
		resourceInfo.ExternalizedAttributes = externalizedAttributeWirePointer(metadata.ExternalizedAttributes)
	}

	if hasResourceInfo(resourceInfo) {
		wire.ResourceInfo = &resourceInfo
	}

	operationInfo := operationInfoWire{}
	if metadata.Filter != nil || metadata.Suppress != nil || strings.TrimSpace(metadata.JQ) != "" {
		operationInfo.Defaults = &operationDefaultsWire{
			Payload: payloadTransformToWire(metadata.Filter, metadata.Suppress, metadata.JQ),
		}
	}
	if metadata.Operations != nil {
		if spec, exists := metadata.Operations[string(OperationGet)]; exists {
			operationInfo.GetResource = operationSpecToWire(OperationGet, spec)
		}
		if spec, exists := metadata.Operations[string(OperationCreate)]; exists {
			operationInfo.CreateResource = operationSpecToWire(OperationCreate, spec)
		}
		if spec, exists := metadata.Operations[string(OperationUpdate)]; exists {
			operationInfo.UpdateResource = operationSpecToWire(OperationUpdate, spec)
		}
		if spec, exists := metadata.Operations[string(OperationDelete)]; exists {
			operationInfo.DeleteResource = operationSpecToWire(OperationDelete, spec)
		}
		if spec, exists := metadata.Operations[string(OperationList)]; exists {
			operationInfo.ListCollection = operationSpecToWire(OperationList, spec)
		}
		if spec, exists := metadata.Operations[string(OperationCompare)]; exists {
			operationInfo.CompareResources = operationSpecToWire(OperationCompare, spec)
		}
	}

	if metadata.Operations != nil || hasOperationInfo(operationInfo) {
		wire.OperationInfo = &operationInfo
	}

	return wire
}

func resourceMetadataFromWire(wire resourceMetadataWire) ResourceMetadata {
	metadata := ResourceMetadata{}

	if wire.ResourceInfo != nil {
		resourceInfo := wire.ResourceInfo
		if resourceInfo.IDFromAttribute != "" {
			metadata.IDFromAttribute = resourceInfo.IDFromAttribute
		}
		if resourceInfo.AliasFromAttribute != "" {
			metadata.AliasFromAttribute = resourceInfo.AliasFromAttribute
		}
		if resourceInfo.CollectionPath != "" {
			metadata.CollectionPath = resourceInfo.CollectionPath
		}
		if resourceInfo.PayloadType != "" {
			metadata.PayloadType = resourceInfo.PayloadType
		}
		if resourceInfo.SecretInAttributes != nil {
			metadata.SecretsFromAttributes = cloneStringSlice(*resourceInfo.SecretInAttributes)
		}
		if resourceInfo.ExternalizedAttributes != nil {
			metadata.ExternalizedAttributes = externalizedAttributesFromWire(*resourceInfo.ExternalizedAttributes)
		}
	}

	if wire.OperationInfo != nil {
		if operationMap := operationInfoToMap(wire.OperationInfo); operationMap != nil {
			if metadata.Operations == nil {
				metadata.Operations = map[string]OperationSpec{}
			}
			for key, spec := range operationMap {
				metadata.Operations[key] = spec
			}
		}
		if wire.OperationInfo.Defaults != nil {
			defaults := wire.OperationInfo.Defaults
			if defaults.Payload != nil {
				metadata.PayloadTransformOrder = cloneStringSlice(defaults.Payload.Order)
				if defaults.Payload.FilterAttributes != nil {
					metadata.Filter = cloneStringListWire(defaults.Payload.FilterAttributes)
				}
				if defaults.Payload.SuppressAttributes != nil {
					metadata.Suppress = cloneStringListWire(defaults.Payload.SuppressAttributes)
				}
				if defaults.Payload.JQExpression != nil {
					metadata.JQ = *defaults.Payload.JQExpression
				}
			}
		}
		if metadata.Operations == nil &&
			wire.OperationInfo.Defaults == nil &&
			operationInfoIsExplicitEmpty(wire.OperationInfo) {
			metadata.Operations = map[string]OperationSpec{}
		}
	}

	return metadata
}

func hasResourceInfo(resourceInfo resourceInfoWire) bool {
	return strings.TrimSpace(resourceInfo.IDFromAttribute) != "" ||
		strings.TrimSpace(resourceInfo.AliasFromAttribute) != "" ||
		strings.TrimSpace(resourceInfo.CollectionPath) != "" ||
		strings.TrimSpace(resourceInfo.PayloadType) != "" ||
		resourceInfo.SecretInAttributes != nil ||
		resourceInfo.ExternalizedAttributes != nil
}

func hasOperationInfo(info operationInfoWire) bool {
	return info.Defaults != nil ||
		info.GetResource != nil ||
		info.CreateResource != nil ||
		info.UpdateResource != nil ||
		info.DeleteResource != nil ||
		info.ListCollection != nil ||
		info.CompareResources != nil
}

func operationInfoIsExplicitEmpty(info *operationInfoWire) bool {
	if info == nil {
		return false
	}

	return info.GetResource == nil &&
		info.CreateResource == nil &&
		info.UpdateResource == nil &&
		info.DeleteResource == nil &&
		info.ListCollection == nil &&
		info.CompareResources == nil
}

func operationInfoToMap(info *operationInfoWire) map[string]OperationSpec {
	if info == nil {
		return nil
	}

	result := map[string]OperationSpec{}

	set := func(operation Operation, spec *resourceOperationWire) {
		if spec == nil {
			return
		}
		result[string(operation)] = operationSpecFromWire(operation, *spec)
	}

	set(OperationGet, info.GetResource)
	set(OperationCreate, info.CreateResource)
	set(OperationUpdate, info.UpdateResource)
	set(OperationDelete, info.DeleteResource)
	set(OperationList, info.ListCollection)
	set(OperationCompare, info.CompareResources)

	if len(result) == 0 {
		return nil
	}
	return result
}

func operationSpecToWire(operation Operation, spec OperationSpec) *resourceOperationWire {
	wire := &resourceOperationWire{
		HTTPMethod: spec.Method,
		Path:       spec.Path,
		Body:       spec.Body,
	}

	if spec.Query != nil {
		wire.Query = stringMapPointer(spec.Query)
	}
	if headers := operationHeadersToWire(spec); headers != nil {
		wire.HTTPHeaders = httpHeaderListPointer(headers)
	}
	if operation == OperationCompare {
		if spec.Filter != nil || spec.Suppress != nil || strings.TrimSpace(spec.JQ) != "" {
			wire.FilterAttributes = stringListWirePointer(spec.Filter)
			wire.SuppressAttributes = stringListWirePointer(spec.Suppress)
			wire.JQExpression = stringPointer(spec.JQ)
		}
	} else {
		wire.Payload = payloadTransformToWire(spec.Filter, spec.Suppress, spec.JQ)
	}
	if spec.Validate != nil {
		wire.Validate = operationValidationToWire(spec.Validate)
	}

	return wire
}

func operationSpecFromWire(operation Operation, spec resourceOperationWire) OperationSpec {
	decoded := OperationSpec{
		Method: spec.HTTPMethod,
		Path:   spec.Path,
		Body:   spec.Body,
	}

	if spec.Query != nil {
		decoded.Query = cloneStringMap(*spec.Query)
	}
	preserveExplicitEmptyHeaders := spec.HTTPHeaders != nil && len(*spec.HTTPHeaders) == 0
	if spec.HTTPHeaders != nil {
		decoded.Headers = httpHeaderListToMap(*spec.HTTPHeaders)
	}
	promoteMediaHeadersFromOperationHeaders(&decoded, preserveExplicitEmptyHeaders)
	applyOperationTransformsFromWire(&decoded, operation == OperationCompare, spec)
	decoded.Validate = operationValidationFromWire(spec.Validate)

	return decoded
}

func applyOperationTransformsFromWire(target *OperationSpec, compare bool, spec resourceOperationWire) {
	if target == nil {
		return
	}

	if compare {
		if spec.FilterAttributes != nil {
			target.Filter = cloneStringListWire(spec.FilterAttributes)
		}

		if spec.SuppressAttributes != nil {
			target.Suppress = cloneStringListWire(spec.SuppressAttributes)
		}

		if spec.JQExpression != nil {
			target.JQ = *spec.JQExpression
		}

		return
	}

	if spec.Payload != nil {
		target.PayloadTransformOrder = cloneStringSlice(spec.Payload.Order)
		if spec.Payload.FilterAttributes != nil {
			target.Filter = cloneStringListWire(spec.Payload.FilterAttributes)
		}
		if spec.Payload.SuppressAttributes != nil {
			target.Suppress = cloneStringListWire(spec.Payload.SuppressAttributes)
		}
		if spec.Payload.JQExpression != nil {
			target.JQ = *spec.Payload.JQExpression
		}
		return
	}
}

func payloadTransformToWire(filter []string, suppress []string, jq string) *payloadTransformWire {
	if filter == nil && suppress == nil && strings.TrimSpace(jq) == "" {
		return nil
	}

	return &payloadTransformWire{
		FilterAttributes:   stringListWireNullPointer(filter),
		SuppressAttributes: stringListWirePointer(suppress),
		JQExpression:       stringPointer(jq),
	}
}

func operationValidationToWire(value *OperationValidationSpec) *operationValidationWire {
	if value == nil {
		return nil
	}

	wire := &operationValidationWire{
		SchemaRef: value.SchemaRef,
	}
	if value.RequiredAttributes != nil {
		wire.RequiredAttributes = stringListWirePointer(value.RequiredAttributes)
	}
	if value.Assertions != nil {
		items := make([]validationAssertionWire, len(value.Assertions))
		for idx, assertion := range value.Assertions {
			items[idx] = validationAssertionToWire(assertion)
		}
		wire.Assertions = &items
	}
	return wire
}

func operationValidationFromWire(value *operationValidationWire) *OperationValidationSpec {
	if value == nil {
		return nil
	}

	decoded := &OperationValidationSpec{
		SchemaRef: value.SchemaRef,
	}
	if value.RequiredAttributes != nil {
		decoded.RequiredAttributes = cloneStringListWire(value.RequiredAttributes)
	}
	if value.Assertions != nil {
		items := make([]ValidationAssertion, len(*value.Assertions))
		for idx, assertion := range *value.Assertions {
			items[idx] = validationAssertionFromWire(assertion)
		}
		decoded.Assertions = items
	}

	if decoded.RequiredAttributes == nil &&
		decoded.Assertions == nil &&
		strings.TrimSpace(decoded.SchemaRef) == "" {
		return nil
	}

	return decoded
}

func validationAssertionToWire(assertion ValidationAssertion) validationAssertionWire {
	return validationAssertionWire(assertion)
}

func validationAssertionFromWire(assertion validationAssertionWire) ValidationAssertion {
	return ValidationAssertion(assertion)
}

func payloadTransformOrderFromJSON(data []byte) []string {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	token, err := decoder.Token()
	if err != nil {
		return nil
	}
	delim, ok := token.(json.Delim)
	if !ok || delim != '{' {
		return nil
	}

	order := make([]string, 0, 3)
	seen := map[string]struct{}{}
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return normalizePayloadTransformOrder(order)
		}
		key, _ := keyToken.(string)
		var discard any
		if err := decoder.Decode(&discard); err != nil {
			return normalizePayloadTransformOrder(order)
		}
		if step, matched := payloadTransformStepForKey(key); matched {
			if _, exists := seen[step]; !exists {
				seen[step] = struct{}{}
				order = append(order, step)
			}
		}
	}

	return normalizePayloadTransformOrder(order)
}

func payloadTransformOrderFromYAMLNode(node *yaml.Node) []string {
	if node == nil {
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return nil
	}

	order := make([]string, 0, 3)
	seen := map[string]struct{}{}
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		keyNode := node.Content[idx]
		if keyNode == nil {
			continue
		}
		if step, matched := payloadTransformStepForKey(strings.TrimSpace(keyNode.Value)); matched {
			if _, exists := seen[step]; exists {
				continue
			}
			seen[step] = struct{}{}
			order = append(order, step)
		}
	}
	return normalizePayloadTransformOrder(order)
}

func httpHeaderListPointer(values map[string]string) *[]httpHeaderWire {
	if values == nil {
		return nil
	}

	keys := sortedMapKeys(values)
	items := make([]httpHeaderWire, 0, len(keys))
	for _, key := range keys {
		items = append(items, httpHeaderWire{
			Name:  key,
			Value: values[key],
		})
	}
	return &items
}

func httpHeaderListToMap(values []httpHeaderWire) map[string]string {
	result := make(map[string]string, len(values))
	for _, item := range values {
		result[item.Name] = item.Value
	}
	return result
}

func operationHeadersToWire(spec OperationSpec) map[string]string {
	headers := cloneStringMap(spec.Headers)
	headers = setOrReplaceHeader(headers, "Accept", spec.Accept)
	headers = setOrReplaceHeader(headers, "Content-Type", spec.ContentType)
	return headers
}

func setOrReplaceHeader(headers map[string]string, name string, value string) map[string]string {
	if strings.TrimSpace(value) == "" {
		return headers
	}
	if headers == nil {
		headers = map[string]string{}
	}
	for key := range headers {
		if strings.EqualFold(strings.TrimSpace(key), name) {
			delete(headers, key)
		}
	}
	headers[name] = value
	return headers
}

func promoteMediaHeadersFromOperationHeaders(target *OperationSpec, preserveExplicitEmptyHeaders bool) {
	if target == nil || target.Headers == nil {
		return
	}

	if value, key, found := lookupHeaderCaseInsensitive(target.Headers, "Accept"); found {
		if strings.TrimSpace(target.Accept) == "" {
			target.Accept = value
		}
		delete(target.Headers, key)
	}
	if value, key, found := lookupHeaderCaseInsensitive(target.Headers, "Content-Type"); found {
		if strings.TrimSpace(target.ContentType) == "" {
			target.ContentType = value
		}
		delete(target.Headers, key)
	}

	if len(target.Headers) == 0 && !preserveExplicitEmptyHeaders {
		target.Headers = nil
	}
}

func lookupHeaderCaseInsensitive(headers map[string]string, name string) (value string, key string, found bool) {
	for currentKey, currentValue := range headers {
		if strings.EqualFold(strings.TrimSpace(currentKey), name) {
			return currentValue, currentKey, true
		}
	}
	return "", "", false
}

func stringPointer(value string) *string {
	return &value
}

func stringSlicePointer(values []string) *[]string {
	if values == nil {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return &cloned
}

func stringMapPointer(values map[string]string) *map[string]string {
	if values == nil {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return &cloned
}

func externalizedAttributeWirePointer(values []ExternalizedAttribute) *[]externalizedAttributeWire {
	if values == nil {
		return nil
	}

	cloned := make([]externalizedAttributeWire, len(values))
	for idx := range values {
		cloned[idx] = externalizedAttributeWire{
			Path:           stringSlicePointer(values[idx].Path),
			File:           values[idx].File,
			Template:       values[idx].Template,
			Mode:           values[idx].Mode,
			SaveBehavior:   values[idx].SaveBehavior,
			RenderBehavior: values[idx].RenderBehavior,
			Enabled:        cloneBoolPointer(values[idx].Enabled),
		}
	}
	return &cloned
}

func externalizedAttributesFromWire(values []externalizedAttributeWire) []ExternalizedAttribute {
	cloned := make([]ExternalizedAttribute, len(values))
	for idx := range values {
		cloned[idx] = ExternalizedAttribute{
			File:           values[idx].File,
			Template:       values[idx].Template,
			Mode:           values[idx].Mode,
			SaveBehavior:   values[idx].SaveBehavior,
			RenderBehavior: values[idx].RenderBehavior,
			Enabled:        cloneBoolPointer(values[idx].Enabled),
		}
		if values[idx].Path != nil {
			cloned[idx].Path = cloneStringSlice(*values[idx].Path)
		}
	}
	return cloned
}
