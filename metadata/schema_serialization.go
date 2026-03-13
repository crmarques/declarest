package metadata

import (
	"bytes"
	"encoding/json"
	"maps"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Wire types below map between the nested JSON/YAML schema shape
// (resource: / operations:) and the flat canonical ResourceMetadata.
// They handle pointer-to-slice conversions for omitempty semantics,
// Accept/Content-Type header promotion, and the stringListWire
// scalar-or-array flexibility. These types must stay in sync with the
// canonical types in types.go; see TestDisplayTypesMatchCanonicalFieldCount
// for drift detection on the display-facing counterparts.
type resourceMetadataWire struct {
	Resource   *resourceWire   `json:"resource,omitempty" yaml:"resource,omitempty"`
	Operations *operationsWire `json:"operations,omitempty" yaml:"operations,omitempty"`
}

type resourceWire struct {
	ID                     string                       `json:"id,omitempty" yaml:"id,omitempty"`
	Alias                  string                       `json:"alias,omitempty" yaml:"alias,omitempty"`
	RequiredAttributes     *[]string                    `json:"requiredAttributes,omitempty" yaml:"requiredAttributes,omitempty"`
	RemoteCollectionPath   string                       `json:"remoteCollectionPath,omitempty" yaml:"remoteCollectionPath,omitempty"`
	PayloadType            string                       `json:"payloadType,omitempty" yaml:"payloadType,omitempty"`
	DefaultFormat          string                       `json:"defaultFormat,omitempty" yaml:"defaultFormat,omitempty"`
	Secret                 *bool                        `json:"secret,omitempty" yaml:"secret,omitempty"`
	SecretAttributes       *[]string                    `json:"secretAttributes,omitempty" yaml:"secretAttributes,omitempty"`
	ExternalizedAttributes *[]externalizedAttributeWire `json:"externalizedAttributes,omitempty" yaml:"externalizedAttributes,omitempty"`
}

type externalizedAttributeWire struct {
	Path           string `json:"path,omitempty" yaml:"path,omitempty"`
	File           string `json:"file,omitempty" yaml:"file,omitempty"`
	Template       string `json:"template,omitempty" yaml:"template,omitempty"`
	Mode           string `json:"mode,omitempty" yaml:"mode,omitempty"`
	SaveBehavior   string `json:"saveBehavior,omitempty" yaml:"saveBehavior,omitempty"`
	RenderBehavior string `json:"renderBehavior,omitempty" yaml:"renderBehavior,omitempty"`
	Enabled        *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type operationsWire struct {
	Defaults *operationDefaultsWire `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	Get      *resourceOperationWire `json:"get,omitempty" yaml:"get,omitempty"`
	Create   *resourceOperationWire `json:"create,omitempty" yaml:"create,omitempty"`
	Update   *resourceOperationWire `json:"update,omitempty" yaml:"update,omitempty"`
	Delete   *resourceOperationWire `json:"delete,omitempty" yaml:"delete,omitempty"`
	List     *resourceOperationWire `json:"list,omitempty" yaml:"list,omitempty"`
	Compare  *resourceOperationWire `json:"compare,omitempty" yaml:"compare,omitempty"`
}

type operationDefaultsWire struct {
	Transforms *[]transformStepWire `json:"transforms,omitempty" yaml:"transforms,omitempty"`
}

type transformStepWire struct {
	SelectAttributes  *stringListWire `json:"selectAttributes,omitempty" yaml:"selectAttributes,omitempty"`
	ExcludeAttributes *stringListWire `json:"excludeAttributes,omitempty" yaml:"excludeAttributes,omitempty"`
	JQExpression      *string         `json:"jqExpression,omitempty" yaml:"jqExpression,omitempty"`
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

type headerMapWire map[string]string

type resourceOperationWire struct {
	Method     string                   `json:"method,omitempty" yaml:"method,omitempty"`
	Path       string                   `json:"path,omitempty" yaml:"path,omitempty"`
	Query      *map[string]string       `json:"query,omitempty" yaml:"query,omitempty"`
	Headers    *headerMapWire           `json:"headers,omitempty" yaml:"headers,omitempty"`
	Body       any                      `json:"body,omitempty" yaml:"body,omitempty"`
	Transforms *[]transformStepWire     `json:"transforms,omitempty" yaml:"transforms,omitempty"`
	Validate   *operationValidationWire `json:"validate,omitempty" yaml:"validate,omitempty"`
}

func (h headerMapWire) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]string(h))
}

func (h *headerMapWire) UnmarshalJSON(data []byte) error {
	decoded, err := decodeHeaderMapWireJSON(data)
	if err != nil {
		return err
	}
	*h = headerMapWire(decoded)
	return nil
}

func (h headerMapWire) MarshalYAML() (any, error) {
	return map[string]string(h), nil
}

func (h *headerMapWire) UnmarshalYAML(value *yaml.Node) error {
	decoded, err := decodeHeaderMapWireYAML(value)
	if err != nil {
		return err
	}
	*h = headerMapWire(decoded)
	return nil
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

func (m *ResourceMetadata) UnmarshalYAML(value *yaml.Node) error {
	var buffer bytes.Buffer
	encoder := yaml.NewEncoder(&buffer)
	if err := encoder.Encode(value); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}

	decoded, err := DecodeResourceMetadataYAML(buffer.Bytes())
	if err != nil {
		return err
	}
	*m = decoded
	return nil
}

func DecodeResourceMetadataJSON(data []byte) (ResourceMetadata, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	wire := resourceMetadataWire{}
	if err := decoder.Decode(&wire); err != nil {
		return ResourceMetadata{}, err
	}

	return resourceMetadataFromWire(wire)
}

func DecodeResourceMetadataYAML(data []byte) (ResourceMetadata, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	wire := resourceMetadataWire{}
	if err := decoder.Decode(&wire); err != nil {
		return ResourceMetadata{}, err
	}

	return resourceMetadataFromWire(wire)
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

func EffectiveRemoteCollectionPath(md ResourceMetadata, fallback string) string {
	if override := strings.TrimSpace(md.RemoteCollectionPath); override != "" {
		return override
	}
	return fallback
}

func resourceMetadataToWire(metadata ResourceMetadata) resourceMetadataWire {
	wire := resourceMetadataWire{}

	resource := resourceWire{
		ID:                   metadata.ID,
		Alias:                metadata.Alias,
		RemoteCollectionPath: metadata.RemoteCollectionPath,
		PayloadType:          metadata.PayloadType,
		DefaultFormat:        metadata.DefaultFormat,
		Secret:               cloneBoolPointer(metadata.Secret),
	}
	if metadata.RequiredAttributes != nil {
		resource.RequiredAttributes = stringSlicePointer(metadata.RequiredAttributes)
	}
	if metadata.SecretAttributes != nil {
		resource.SecretAttributes = stringSlicePointer(metadata.SecretAttributes)
	}
	if metadata.ExternalizedAttributes != nil {
		resource.ExternalizedAttributes = externalizedAttributeWirePointer(metadata.ExternalizedAttributes)
	}

	if hasResourceInfo(resource) {
		wire.Resource = &resource
	}

	operations := operationsWire{}
	if metadata.Transforms != nil {
		operations.Defaults = &operationDefaultsWire{
			Transforms: transformStepsWirePointer(metadata.Transforms),
		}
	}
	if metadata.Operations != nil {
		if spec, exists := metadata.Operations[string(OperationGet)]; exists {
			operations.Get = operationSpecToWire(OperationGet, spec)
		}
		if spec, exists := metadata.Operations[string(OperationCreate)]; exists {
			operations.Create = operationSpecToWire(OperationCreate, spec)
		}
		if spec, exists := metadata.Operations[string(OperationUpdate)]; exists {
			operations.Update = operationSpecToWire(OperationUpdate, spec)
		}
		if spec, exists := metadata.Operations[string(OperationDelete)]; exists {
			operations.Delete = operationSpecToWire(OperationDelete, spec)
		}
		if spec, exists := metadata.Operations[string(OperationList)]; exists {
			operations.List = operationSpecToWire(OperationList, spec)
		}
		if spec, exists := metadata.Operations[string(OperationCompare)]; exists {
			operations.Compare = operationSpecToWire(OperationCompare, spec)
		}
	}

	if metadata.Operations != nil || hasOperationsInfo(operations) {
		wire.Operations = &operations
	}

	return wire
}

func resourceMetadataFromWire(wire resourceMetadataWire) (ResourceMetadata, error) {
	metadata := ResourceMetadata{}

	if wire.Resource != nil {
		resource := wire.Resource
		if resource.ID != "" {
			metadata.ID = resource.ID
		}
		if resource.Alias != "" {
			metadata.Alias = resource.Alias
		}
		if resource.RequiredAttributes != nil {
			metadata.RequiredAttributes = cloneStringSlice(*resource.RequiredAttributes)
		}
		remoteCollectionPath := strings.TrimSpace(resource.RemoteCollectionPath)
		if remoteCollectionPath != "" {
			metadata.RemoteCollectionPath = remoteCollectionPath
		}
		if resource.PayloadType != "" {
			metadata.PayloadType = resource.PayloadType
		}
		if resource.DefaultFormat != "" {
			metadata.DefaultFormat = resource.DefaultFormat
		}
		if resource.Secret != nil {
			metadata.Secret = cloneBoolPointer(resource.Secret)
		}
		if resource.SecretAttributes != nil {
			metadata.SecretAttributes = cloneStringSlice(*resource.SecretAttributes)
		}
		if resource.ExternalizedAttributes != nil {
			metadata.ExternalizedAttributes = externalizedAttributesFromWire(*resource.ExternalizedAttributes)
		}
	}

	if wire.Operations != nil {
		if operationMap := operationsToMap(wire.Operations); operationMap != nil {
			if metadata.Operations == nil {
				metadata.Operations = map[string]OperationSpec{}
			}
			for key, spec := range operationMap {
				metadata.Operations[key] = spec
			}
		}
		if wire.Operations.Defaults != nil {
			defaults := wire.Operations.Defaults
			if defaults.Transforms != nil {
				metadata.Transforms = transformStepsFromWire(*defaults.Transforms)
			}
		}
		if metadata.Operations == nil &&
			wire.Operations.Defaults == nil &&
			operationsIsExplicitEmpty(wire.Operations) {
			metadata.Operations = map[string]OperationSpec{}
		}
	}

	return metadata, nil
}

func hasResourceInfo(resource resourceWire) bool {
	return strings.TrimSpace(resource.ID) != "" ||
		strings.TrimSpace(resource.Alias) != "" ||
		resource.RequiredAttributes != nil ||
		strings.TrimSpace(resource.RemoteCollectionPath) != "" ||
		strings.TrimSpace(resource.PayloadType) != "" ||
		strings.TrimSpace(resource.DefaultFormat) != "" ||
		resource.Secret != nil ||
		resource.SecretAttributes != nil ||
		resource.ExternalizedAttributes != nil
}

func hasOperationsInfo(info operationsWire) bool {
	return info.Defaults != nil ||
		info.Get != nil ||
		info.Create != nil ||
		info.Update != nil ||
		info.Delete != nil ||
		info.List != nil ||
		info.Compare != nil
}

func operationsIsExplicitEmpty(info *operationsWire) bool {
	if info == nil {
		return false
	}

	return info.Get == nil &&
		info.Create == nil &&
		info.Update == nil &&
		info.Delete == nil &&
		info.List == nil &&
		info.Compare == nil
}

func operationsToMap(info *operationsWire) map[string]OperationSpec {
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

	set(OperationGet, info.Get)
	set(OperationCreate, info.Create)
	set(OperationUpdate, info.Update)
	set(OperationDelete, info.Delete)
	set(OperationList, info.List)
	set(OperationCompare, info.Compare)

	if len(result) == 0 {
		return nil
	}
	return result
}

func operationSpecToWire(_ Operation, spec OperationSpec) *resourceOperationWire {
	wire := &resourceOperationWire{
		Method:   spec.Method,
		Path:     spec.Path,
		Body:     spec.Body,
		Validate: operationValidationToWire(spec.Validate),
	}

	if spec.Query != nil {
		wire.Query = stringMapPointer(spec.Query)
	}
	if headers := operationHeadersToWire(spec); headers != nil {
		wire.Headers = headerMapWirePointer(headers)
	}
	if spec.Transforms != nil {
		wire.Transforms = transformStepsWirePointer(spec.Transforms)
	}

	return wire
}

func operationSpecFromWire(_ Operation, spec resourceOperationWire) OperationSpec {
	decoded := OperationSpec{
		Method: spec.Method,
		Path:   spec.Path,
		Body:   spec.Body,
	}

	if spec.Query != nil {
		decoded.Query = maps.Clone(*spec.Query)
	}
	preserveExplicitEmptyHeaders := spec.Headers != nil && len(*spec.Headers) == 0
	if spec.Headers != nil {
		decoded.Headers = maps.Clone(map[string]string(*spec.Headers))
	}
	promoteMediaHeadersFromOperationHeaders(&decoded, preserveExplicitEmptyHeaders)
	if spec.Transforms != nil {
		decoded.Transforms = transformStepsFromWire(*spec.Transforms)
	}
	decoded.Validate = operationValidationFromWire(spec.Validate)

	return decoded
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

func transformStepsWirePointer(values []TransformStep) *[]transformStepWire {
	if values == nil {
		return nil
	}

	items := make([]transformStepWire, len(values))
	for idx, value := range values {
		items[idx] = transformStepToWire(value)
	}
	return &items
}

func transformStepsFromWire(values []transformStepWire) []TransformStep {
	decoded := make([]TransformStep, len(values))
	for idx, value := range values {
		decoded[idx] = transformStepFromWire(value)
	}
	return decoded
}

func transformStepToWire(value TransformStep) transformStepWire {
	return transformStepWire{
		SelectAttributes:  stringListWireNullPointer(value.SelectAttributes),
		ExcludeAttributes: stringListWireNullPointer(value.ExcludeAttributes),
		JQExpression:      stringPointer(value.JQExpression),
	}
}

func transformStepFromWire(value transformStepWire) TransformStep {
	return TransformStep{
		SelectAttributes:  cloneStringListWire(value.SelectAttributes),
		ExcludeAttributes: cloneStringListWire(value.ExcludeAttributes),
		JQExpression:      stringValue(value.JQExpression),
	}
}

func operationHeadersToWire(spec OperationSpec) map[string]string {
	headers := maps.Clone(spec.Headers)
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

func decodeHeaderMapWireJSON(data []byte) (map[string]string, error) {
	var object map[string]string
	if err := json.Unmarshal(data, &object); err == nil {
		return object, nil
	}

	var list []map[string]string
	if err := json.Unmarshal(data, &list); err == nil {
		return mergeHeaderMaps(list), nil
	}

	return nil, json.Unmarshal(data, &object)
}

func decodeHeaderMapWireYAML(value *yaml.Node) (map[string]string, error) {
	var object map[string]string
	if err := value.Decode(&object); err == nil {
		return object, nil
	}

	var list []map[string]string
	if err := value.Decode(&list); err == nil {
		return mergeHeaderMaps(list), nil
	}

	return nil, value.Decode(&object)
}

func mergeHeaderMaps(values []map[string]string) map[string]string {
	if values == nil {
		return nil
	}

	merged := map[string]string{}
	for _, item := range values {
		for key, value := range item {
			merged[key] = value
		}
	}
	return merged
}

func externalizedAttributeWirePointer(values []ExternalizedAttribute) *[]externalizedAttributeWire {
	if values == nil {
		return nil
	}

	cloned := make([]externalizedAttributeWire, len(values))
	for idx := range values {
		cloned[idx] = externalizedAttributeWire{
			Path:           values[idx].Path,
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
			Path:           values[idx].Path,
			File:           values[idx].File,
			Template:       values[idx].Template,
			Mode:           values[idx].Mode,
			SaveBehavior:   values[idx].SaveBehavior,
			RenderBehavior: values[idx].RenderBehavior,
			Enabled:        cloneBoolPointer(values[idx].Enabled),
		}
	}
	return cloned
}
