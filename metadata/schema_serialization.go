package metadata

import (
	"bytes"
	"encoding/json"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	operationInfoGetResourceKey      = "getResource"
	operationInfoCreateResourceKey   = "createResource"
	operationInfoUpdateResourceKey   = "updateResource"
	operationInfoDeleteResourceKey   = "deleteResource"
	operationInfoListCollectionKey   = "listCollection"
	operationInfoCompareResourcesKey = "compareResources"
)

type resourceMetadataWire struct {
	ResourceInfo  *resourceInfoWire  `json:"resourceInfo,omitempty" yaml:"resourceInfo,omitempty"`
	OperationInfo *operationInfoWire `json:"operationInfo,omitempty" yaml:"operationInfo,omitempty"`

	// Backward-compatible flat fields.
	IDFromAttribute       string                            `json:"idFromAttribute,omitempty" yaml:"idFromAttribute,omitempty"`
	AliasFromAttribute    string                            `json:"aliasFromAttribute,omitempty" yaml:"aliasFromAttribute,omitempty"`
	CollectionPath        string                            `json:"collectionPath,omitempty" yaml:"collectionPath,omitempty"`
	SecretsFromAttributes *[]string                         `json:"secretsFromAttributes,omitempty" yaml:"secretsFromAttributes,omitempty"`
	SecretInAttributes    *[]string                         `json:"secretInAttributes,omitempty" yaml:"secretInAttributes,omitempty"`
	Operations            *map[string]resourceOperationWire `json:"operations,omitempty" yaml:"operations,omitempty"`
	Filter                *[]string                         `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress              *[]string                         `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ                    string                            `json:"jq,omitempty" yaml:"jq,omitempty"`
}

type resourceInfoWire struct {
	IDFromAttribute       string    `json:"idFromAttribute,omitempty" yaml:"idFromAttribute,omitempty"`
	AliasFromAttribute    string    `json:"aliasFromAttribute,omitempty" yaml:"aliasFromAttribute,omitempty"`
	CollectionPath        string    `json:"collectionPath,omitempty" yaml:"collectionPath,omitempty"`
	SecretInAttributes    *[]string `json:"secretInAttributes,omitempty" yaml:"secretInAttributes,omitempty"`
	SecretsFromAttributes *[]string `json:"secretsFromAttributes,omitempty" yaml:"secretsFromAttributes,omitempty"`
}

type operationInfoWire struct {
	Defaults         *operationDefaultsWire `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	GetResource      *resourceOperationWire `json:"getResource,omitempty" yaml:"getResource,omitempty"`
	CreateResource   *resourceOperationWire `json:"createResource,omitempty" yaml:"createResource,omitempty"`
	UpdateResource   *resourceOperationWire `json:"updateResource,omitempty" yaml:"updateResource,omitempty"`
	DeleteResource   *resourceOperationWire `json:"deleteResource,omitempty" yaml:"deleteResource,omitempty"`
	ListCollection   *resourceOperationWire `json:"listCollection,omitempty" yaml:"listCollection,omitempty"`
	CompareResources *resourceOperationWire `json:"compareResources,omitempty" yaml:"compareResources,omitempty"`

	// Backward-compatible operation names.
	Get     *resourceOperationWire `json:"get,omitempty" yaml:"get,omitempty"`
	Create  *resourceOperationWire `json:"create,omitempty" yaml:"create,omitempty"`
	Update  *resourceOperationWire `json:"update,omitempty" yaml:"update,omitempty"`
	Delete  *resourceOperationWire `json:"delete,omitempty" yaml:"delete,omitempty"`
	List    *resourceOperationWire `json:"list,omitempty" yaml:"list,omitempty"`
	Compare *resourceOperationWire `json:"compare,omitempty" yaml:"compare,omitempty"`
}

type operationDefaultsWire struct {
	Payload *payloadTransformWire `json:"payload,omitempty" yaml:"payload,omitempty"`

	// Backward-compatible transform names.
	Filter   *[]string `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress *[]string `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ       *string   `json:"jq,omitempty" yaml:"jq,omitempty"`
}

type payloadTransformWire struct {
	FilterAttributes   *stringListWire `json:"filterAttributes,omitempty" yaml:"filterAttributes,omitempty"`
	SuppressAttributes *stringListWire `json:"suppressAttributes,omitempty" yaml:"suppressAttributes,omitempty"`
	JQExpression       *string         `json:"jqExpression,omitempty" yaml:"jqExpression,omitempty"`

	// Backward-compatible transform names.
	Filter   *[]string `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress *[]string `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ       *string   `json:"jq,omitempty" yaml:"jq,omitempty"`

	Order []string `json:"-" yaml:"-"`
}

type httpHeaderWire struct {
	Name  string `json:"name" yaml:"name"`
	Value string `json:"value" yaml:"value"`
}

type resourceOperationWire struct {
	HTTPMethod  string                `json:"httpMethod,omitempty" yaml:"httpMethod,omitempty"`
	Path        string                `json:"path,omitempty" yaml:"path,omitempty"`
	URL         *resourceURLWire      `json:"url,omitempty" yaml:"url,omitempty"`
	Query       *map[string]string    `json:"query,omitempty" yaml:"query,omitempty"`
	HTTPHeaders *[]httpHeaderWire     `json:"httpHeaders,omitempty" yaml:"httpHeaders,omitempty"`
	Body        any                   `json:"body,omitempty" yaml:"body,omitempty"`
	Payload     *payloadTransformWire `json:"payload,omitempty" yaml:"payload,omitempty"`

	// CompareResources transform fields (ignoreAttributes kept for compatibility decode).
	IgnoreAttributes   *stringListWire `json:"ignoreAttributes,omitempty" yaml:"ignoreAttributes,omitempty"`
	FilterAttributes   *stringListWire `json:"filterAttributes,omitempty" yaml:"filterAttributes,omitempty"`
	SuppressAttributes *stringListWire `json:"suppressAttributes,omitempty" yaml:"suppressAttributes,omitempty"`
	JQExpression       *string         `json:"jqExpression,omitempty" yaml:"jqExpression,omitempty"`

	// Backward-compatible operation names.
	Method      string             `json:"method,omitempty" yaml:"method,omitempty"`
	Headers     *map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Accept      string             `json:"accept,omitempty" yaml:"accept,omitempty"`
	ContentType string             `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Filter      *[]string          `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress    *[]string          `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ          *string            `json:"jq,omitempty" yaml:"jq,omitempty"`
}

type resourceURLWire struct {
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
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
	}
	if metadata.SecretsFromAttributes != nil {
		resourceInfo.SecretInAttributes = stringSlicePointer(metadata.SecretsFromAttributes)
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
	metadata := ResourceMetadata{
		IDFromAttribute:    wire.IDFromAttribute,
		AliasFromAttribute: wire.AliasFromAttribute,
		CollectionPath:     wire.CollectionPath,
		JQ:                 wire.JQ,
	}

	if wire.SecretInAttributes != nil {
		metadata.SecretsFromAttributes = cloneStringSlice(*wire.SecretInAttributes)
	} else if wire.SecretsFromAttributes != nil {
		metadata.SecretsFromAttributes = cloneStringSlice(*wire.SecretsFromAttributes)
	}
	if wire.Operations != nil {
		metadata.Operations = make(map[string]OperationSpec, len(*wire.Operations))
		for key, spec := range *wire.Operations {
			metadata.Operations[key] = operationSpecFromWire(Operation(key), spec)
		}
	}
	if wire.Filter != nil {
		metadata.Filter = cloneStringSlice(*wire.Filter)
	}
	if wire.Suppress != nil {
		metadata.Suppress = cloneStringSlice(*wire.Suppress)
	}

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
		if resourceInfo.SecretInAttributes != nil {
			metadata.SecretsFromAttributes = cloneStringSlice(*resourceInfo.SecretInAttributes)
		} else if resourceInfo.SecretsFromAttributes != nil {
			metadata.SecretsFromAttributes = cloneStringSlice(*resourceInfo.SecretsFromAttributes)
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
				} else if defaults.Payload.Filter != nil {
					metadata.Filter = cloneStringSlice(*defaults.Payload.Filter)
				}
				if defaults.Payload.SuppressAttributes != nil {
					metadata.Suppress = cloneStringListWire(defaults.Payload.SuppressAttributes)
				} else if defaults.Payload.Suppress != nil {
					metadata.Suppress = cloneStringSlice(*defaults.Payload.Suppress)
				}
				if defaults.Payload.JQExpression != nil {
					metadata.JQ = *defaults.Payload.JQExpression
				} else if defaults.Payload.JQ != nil {
					metadata.JQ = *defaults.Payload.JQ
				}
			} else {
				if defaults.Filter != nil {
					metadata.Filter = cloneStringSlice(*defaults.Filter)
				}
				if defaults.Suppress != nil {
					metadata.Suppress = cloneStringSlice(*defaults.Suppress)
				}
				if defaults.JQ != nil {
					metadata.JQ = *defaults.JQ
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
		resourceInfo.SecretInAttributes != nil
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
		info.CompareResources == nil &&
		info.Get == nil &&
		info.Create == nil &&
		info.Update == nil &&
		info.Delete == nil &&
		info.List == nil &&
		info.Compare == nil
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

	// Legacy names are loaded first so new names win when both are present.
	set(OperationGet, info.Get)
	set(OperationCreate, info.Create)
	set(OperationUpdate, info.Update)
	set(OperationDelete, info.Delete)
	set(OperationList, info.List)
	set(OperationCompare, info.Compare)

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

	return wire
}

func operationSpecFromWire(operation Operation, spec resourceOperationWire) OperationSpec {
	pathValue := spec.Path
	if strings.TrimSpace(pathValue) == "" && spec.URL != nil {
		pathValue = spec.URL.Path
	}
	methodValue := spec.HTTPMethod
	if strings.TrimSpace(methodValue) == "" {
		methodValue = spec.Method
	}

	decoded := OperationSpec{
		Method:      methodValue,
		Path:        pathValue,
		Accept:      spec.Accept,
		ContentType: spec.ContentType,
		Body:        spec.Body,
	}

	if spec.Query != nil {
		decoded.Query = cloneStringMap(*spec.Query)
	}
	if spec.HTTPHeaders != nil {
		decoded.Headers = httpHeaderListToMap(*spec.HTTPHeaders)
	} else if spec.Headers != nil {
		decoded.Headers = cloneStringMap(*spec.Headers)
	}
	preserveExplicitEmptyHeaders := false
	if spec.HTTPHeaders != nil {
		preserveExplicitEmptyHeaders = len(*spec.HTTPHeaders) == 0
	} else if spec.Headers != nil {
		preserveExplicitEmptyHeaders = len(*spec.Headers) == 0
	}
	promoteMediaHeadersFromOperationHeaders(&decoded, preserveExplicitEmptyHeaders)
	applyOperationTransformsFromWire(&decoded, operation == OperationCompare, spec)

	return decoded
}

func applyOperationTransformsFromWire(target *OperationSpec, compare bool, spec resourceOperationWire) {
	if target == nil {
		return
	}

	if compare {
		if spec.FilterAttributes != nil {
			target.Filter = cloneStringListWire(spec.FilterAttributes)
		} else if spec.Payload != nil {
			if spec.Payload.FilterAttributes != nil {
				target.Filter = cloneStringListWire(spec.Payload.FilterAttributes)
			} else if spec.Payload.Filter != nil {
				target.Filter = cloneStringSlice(*spec.Payload.Filter)
			}
		} else if spec.Filter != nil {
			target.Filter = cloneStringSlice(*spec.Filter)
		}

		if spec.SuppressAttributes != nil {
			target.Suppress = cloneStringListWire(spec.SuppressAttributes)
		} else if spec.IgnoreAttributes != nil {
			target.Suppress = cloneStringListWire(spec.IgnoreAttributes)
		} else if spec.Payload != nil {
			if spec.Payload.SuppressAttributes != nil {
				target.Suppress = cloneStringListWire(spec.Payload.SuppressAttributes)
			} else if spec.Payload.Suppress != nil {
				target.Suppress = cloneStringSlice(*spec.Payload.Suppress)
			}
		} else if spec.Suppress != nil {
			target.Suppress = cloneStringSlice(*spec.Suppress)
		}

		if spec.JQExpression != nil {
			target.JQ = *spec.JQExpression
		} else if spec.Payload != nil {
			if spec.Payload.JQExpression != nil {
				target.JQ = *spec.Payload.JQExpression
			} else if spec.Payload.JQ != nil {
				target.JQ = *spec.Payload.JQ
			}
		} else if spec.JQ != nil {
			target.JQ = *spec.JQ
		}
		if spec.Payload != nil && len(spec.Payload.Order) > 0 {
			target.PayloadTransformOrder = cloneStringSlice(spec.Payload.Order)
		}

		return
	}

	if spec.Payload != nil {
		target.PayloadTransformOrder = cloneStringSlice(spec.Payload.Order)
		if spec.Payload.FilterAttributes != nil {
			target.Filter = cloneStringListWire(spec.Payload.FilterAttributes)
		} else if spec.Payload.Filter != nil {
			target.Filter = cloneStringSlice(*spec.Payload.Filter)
		}
		if spec.Payload.SuppressAttributes != nil {
			target.Suppress = cloneStringListWire(spec.Payload.SuppressAttributes)
		} else if spec.Payload.Suppress != nil {
			target.Suppress = cloneStringSlice(*spec.Payload.Suppress)
		}
		if spec.Payload.JQExpression != nil {
			target.JQ = *spec.Payload.JQExpression
		} else if spec.Payload.JQ != nil {
			target.JQ = *spec.Payload.JQ
		}
		return
	}

	if spec.Filter != nil {
		target.Filter = cloneStringSlice(*spec.Filter)
	}
	if spec.Suppress != nil {
		target.Suppress = cloneStringSlice(*spec.Suppress)
	}
	if spec.JQ != nil {
		target.JQ = *spec.JQ
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
