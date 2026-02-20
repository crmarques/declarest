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
	Filter   *[]string `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress *[]string `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ       string    `json:"jq,omitempty" yaml:"jq,omitempty"`
}

type resourceOperationWire struct {
	Method      string             `json:"method,omitempty" yaml:"method,omitempty"`
	Path        string             `json:"path,omitempty" yaml:"path,omitempty"`
	URL         *resourceURLWire   `json:"url,omitempty" yaml:"url,omitempty"`
	Query       *map[string]string `json:"query,omitempty" yaml:"query,omitempty"`
	Headers     *map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Accept      string             `json:"accept,omitempty" yaml:"accept,omitempty"`
	ContentType string             `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Body        any                `json:"body,omitempty" yaml:"body,omitempty"`
	Filter      *[]string          `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress    *[]string          `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ          string             `json:"jq,omitempty" yaml:"jq,omitempty"`
}

type resourceURLWire struct {
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

func (m ResourceMetadata) MarshalJSON() ([]byte, error) {
	wire := resourceMetadataToWire(m)
	return json.Marshal(wire)
}

func (m *ResourceMetadata) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	wire := resourceMetadataWire{}
	if err := decoder.Decode(&wire); err != nil {
		return err
	}

	*m = resourceMetadataFromWire(wire)
	return nil
}

func (m ResourceMetadata) MarshalYAML() (any, error) {
	return resourceMetadataToWire(m), nil
}

func (m *ResourceMetadata) UnmarshalYAML(value *yaml.Node) error {
	wire := resourceMetadataWire{}
	if err := value.Decode(&wire); err != nil {
		return err
	}

	*m = resourceMetadataFromWire(wire)
	return nil
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
			Filter:   stringSlicePointer(metadata.Filter),
			Suppress: stringSlicePointer(metadata.Suppress),
			JQ:       metadata.JQ,
		}
	}
	if metadata.Operations != nil {
		if spec, exists := metadata.Operations[string(OperationGet)]; exists {
			operationInfo.GetResource = operationSpecToWire(spec)
		}
		if spec, exists := metadata.Operations[string(OperationCreate)]; exists {
			operationInfo.CreateResource = operationSpecToWire(spec)
		}
		if spec, exists := metadata.Operations[string(OperationUpdate)]; exists {
			operationInfo.UpdateResource = operationSpecToWire(spec)
		}
		if spec, exists := metadata.Operations[string(OperationDelete)]; exists {
			operationInfo.DeleteResource = operationSpecToWire(spec)
		}
		if spec, exists := metadata.Operations[string(OperationList)]; exists {
			operationInfo.ListCollection = operationSpecToWire(spec)
		}
		if spec, exists := metadata.Operations[string(OperationCompare)]; exists {
			operationInfo.CompareResources = operationSpecToWire(spec)
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
			metadata.Operations[key] = operationSpecFromWire(spec)
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
			if defaults.Filter != nil {
				metadata.Filter = cloneStringSlice(*defaults.Filter)
			}
			if defaults.Suppress != nil {
				metadata.Suppress = cloneStringSlice(*defaults.Suppress)
			}
			if defaults.JQ != "" {
				metadata.JQ = defaults.JQ
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
		result[string(operation)] = operationSpecFromWire(*spec)
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

func operationSpecToWire(spec OperationSpec) *resourceOperationWire {
	wire := &resourceOperationWire{
		Method:      spec.Method,
		Path:        spec.Path,
		Accept:      spec.Accept,
		ContentType: spec.ContentType,
		Body:        spec.Body,
		JQ:          spec.JQ,
	}

	if spec.Query != nil {
		wire.Query = stringMapPointer(spec.Query)
	}
	if spec.Headers != nil {
		wire.Headers = stringMapPointer(spec.Headers)
	}
	if spec.Filter != nil {
		wire.Filter = stringSlicePointer(spec.Filter)
	}
	if spec.Suppress != nil {
		wire.Suppress = stringSlicePointer(spec.Suppress)
	}

	return wire
}

func operationSpecFromWire(spec resourceOperationWire) OperationSpec {
	pathValue := spec.Path
	if strings.TrimSpace(pathValue) == "" && spec.URL != nil {
		pathValue = spec.URL.Path
	}

	operation := OperationSpec{
		Method:      spec.Method,
		Path:        pathValue,
		Accept:      spec.Accept,
		ContentType: spec.ContentType,
		Body:        spec.Body,
		JQ:          spec.JQ,
	}

	if spec.Query != nil {
		operation.Query = cloneStringMap(*spec.Query)
	}
	if spec.Headers != nil {
		operation.Headers = cloneStringMap(*spec.Headers)
	}
	if spec.Filter != nil {
		operation.Filter = cloneStringSlice(*spec.Filter)
	}
	if spec.Suppress != nil {
		operation.Suppress = cloneStringSlice(*spec.Suppress)
	}

	return operation
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
