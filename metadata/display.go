package metadata

import "maps"

type displayResourceMetadata struct {
	Resource   displayResourceWire   `json:"resource" yaml:"resource"`
	Operations displayOperationsWire `json:"operations" yaml:"operations"`
}

type displayResourceWire struct {
	ID                     string                             `json:"id" yaml:"id"`
	Alias                  string                             `json:"alias" yaml:"alias"`
	RequiredAttributes     []string                           `json:"requiredAttributes" yaml:"requiredAttributes"`
	RemoteCollectionPath   string                             `json:"remoteCollectionPath" yaml:"remoteCollectionPath"`
	PayloadType            string                             `json:"payloadType" yaml:"payloadType"`
	DefaultFormat          string                             `json:"defaultFormat" yaml:"defaultFormat"`
	Secret                 bool                               `json:"secret" yaml:"secret"`
	SecretAttributes       []string                           `json:"secretAttributes" yaml:"secretAttributes"`
	ExternalizedAttributes []displayExternalizedAttributeWire `json:"externalizedAttributes" yaml:"externalizedAttributes"`
}

type displayExternalizedAttributeWire struct {
	Path           string `json:"path" yaml:"path"`
	File           string `json:"file" yaml:"file"`
	Template       string `json:"template" yaml:"template"`
	Mode           string `json:"mode" yaml:"mode"`
	SaveBehavior   string `json:"saveBehavior" yaml:"saveBehavior"`
	RenderBehavior string `json:"renderBehavior" yaml:"renderBehavior"`
	Enabled        bool   `json:"enabled" yaml:"enabled"`
}

type displayOperationsWire struct {
	Defaults displayOperationDefaultsWire `json:"defaults" yaml:"defaults"`
	Get      displayOperationWire         `json:"get" yaml:"get"`
	Create   displayOperationWire         `json:"create" yaml:"create"`
	Update   displayOperationWire         `json:"update" yaml:"update"`
	Delete   displayOperationWire         `json:"delete" yaml:"delete"`
	List     displayOperationWire         `json:"list" yaml:"list"`
	Compare  displayOperationWire         `json:"compare" yaml:"compare"`
}

type displayOperationDefaultsWire struct {
	Transforms []displayTransformStepWire `json:"transforms" yaml:"transforms"`
}

type displayOperationWire struct {
	Method     string                         `json:"method" yaml:"method"`
	Path       string                         `json:"path" yaml:"path"`
	Query      map[string]string              `json:"query" yaml:"query"`
	Headers    map[string]string              `json:"headers" yaml:"headers"`
	Body       any                            `json:"body" yaml:"body"`
	Transforms []displayTransformStepWire     `json:"transforms" yaml:"transforms"`
	Validate   displayOperationValidationWire `json:"validate" yaml:"validate"`
}

type displayTransformStepWire struct {
	SelectAttributes  []string `json:"selectAttributes" yaml:"selectAttributes"`
	ExcludeAttributes []string `json:"excludeAttributes" yaml:"excludeAttributes"`
	JQExpression      string   `json:"jqExpression" yaml:"jqExpression"`
}

type displayOperationValidationWire struct {
	RequiredAttributes []string                         `json:"requiredAttributes" yaml:"requiredAttributes"`
	Assertions         []displayValidationAssertionWire `json:"assertions" yaml:"assertions"`
	SchemaRef          string                           `json:"schemaRef" yaml:"schemaRef"`
}

type displayValidationAssertionWire struct {
	Message string `json:"message" yaml:"message"`
	JQ      string `json:"jq" yaml:"jq"`
}

// DisplayResourceMetadataView expands metadata into the full CLI-facing shape
// used by `metadata get`, including explicit zero/default values for unset
// attributes while preserving the canonical nested resource/operations schema.
func DisplayResourceMetadataView(value ResourceMetadata) displayResourceMetadata {
	expanded := MergeResourceMetadata(DefaultResourceMetadata(), value)

	return displayResourceMetadata{
		Resource: displayResourceWire{
			ID:                     expanded.ID,
			Alias:                  expanded.Alias,
			RequiredAttributes:     cloneStringSliceOrEmpty(expanded.RequiredAttributes),
			RemoteCollectionPath:   expanded.RemoteCollectionPath,
			PayloadType:            expanded.PayloadType,
			DefaultFormat:          expanded.DefaultFormat,
			Secret:                 expanded.IsWholeResourceSecret(),
			SecretAttributes:       cloneStringSliceOrEmpty(expanded.SecretAttributes),
			ExternalizedAttributes: displayExternalizedAttributes(expanded.ExternalizedAttributes),
		},
		Operations: displayOperationsWire{
			Defaults: displayOperationDefaultsWire{
				Transforms: displayTransformSteps(expanded.Transforms),
			},
			Get:     displayOperation(expanded, OperationGet),
			Create:  displayOperation(expanded, OperationCreate),
			Update:  displayOperation(expanded, OperationUpdate),
			Delete:  displayOperation(expanded, OperationDelete),
			List:    displayOperation(expanded, OperationList),
			Compare: displayOperation(expanded, OperationCompare),
		},
	}
}

func displayOperation(metadata ResourceMetadata, operation Operation) displayOperationWire {
	spec := metadata.Operations[string(operation)]
	wire := operationSpecToWire(operation, spec)

	query := map[string]string{}
	if wire != nil && wire.Query != nil {
		query = maps.Clone(*wire.Query)
	}

	headers := map[string]string{}
	if wire != nil && wire.Headers != nil {
		headers = maps.Clone(map[string]string(*wire.Headers))
	}

	body := any(nil)
	if wire != nil {
		body = wire.Body
	}

	return displayOperationWire{
		Method:     spec.Method,
		Path:       spec.Path,
		Query:      query,
		Headers:    headers,
		Body:       body,
		Transforms: displayTransformSteps(spec.Transforms),
		Validate:   displayOperationValidation(spec.Validate),
	}
}

func displayTransformSteps(values []TransformStep) []displayTransformStepWire {
	if len(values) == 0 {
		return []displayTransformStepWire{}
	}

	items := make([]displayTransformStepWire, len(values))
	for idx, value := range values {
		items[idx] = displayTransformStepWire{
			SelectAttributes:  cloneStringSliceOrEmpty(value.SelectAttributes),
			ExcludeAttributes: cloneStringSliceOrEmpty(value.ExcludeAttributes),
			JQExpression:      value.JQExpression,
		}
	}
	return items
}

func displayOperationValidation(value *OperationValidationSpec) displayOperationValidationWire {
	if value == nil {
		return displayOperationValidationWire{
			RequiredAttributes: []string{},
			Assertions:         []displayValidationAssertionWire{},
			SchemaRef:          "",
		}
	}

	assertions := make([]displayValidationAssertionWire, len(value.Assertions))
	for idx, assertion := range value.Assertions {
		assertions[idx] = displayValidationAssertionWire(assertion)
	}

	return displayOperationValidationWire{
		RequiredAttributes: cloneStringSliceOrEmpty(value.RequiredAttributes),
		Assertions:         assertions,
		SchemaRef:          value.SchemaRef,
	}
}

func displayExternalizedAttributes(values []ExternalizedAttribute) []displayExternalizedAttributeWire {
	if len(values) == 0 {
		return []displayExternalizedAttributeWire{}
	}

	items := make([]displayExternalizedAttributeWire, len(values))
	for idx, value := range values {
		items[idx] = displayExternalizedAttributeWire{
			Path:           value.Path,
			File:           value.File,
			Template:       value.Template,
			Mode:           value.Mode,
			SaveBehavior:   value.SaveBehavior,
			RenderBehavior: value.RenderBehavior,
			Enabled:        value.Enabled != nil && *value.Enabled,
		}
	}
	return items
}
