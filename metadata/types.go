package metadata

type Operation string

const (
	OperationGet     Operation = "get"
	OperationCreate  Operation = "create"
	OperationUpdate  Operation = "update"
	OperationDelete  Operation = "delete"
	OperationList    Operation = "list"
	OperationCompare Operation = "compare"
)

type InferenceRequest struct {
	Apply     bool
	Recursive bool
}

type SelectorSpec struct {
	Descendants *bool `json:"descendants,omitempty" yaml:"descendants,omitempty"`
}

func (s *SelectorSpec) AllowsDescendants() bool {
	return s != nil && s.Descendants != nil && *s.Descendants
}

type ResourceMetadata struct {
	Selector               *SelectorSpec            `json:"selector,omitempty" yaml:"selector,omitempty"`
	ID                     string                   `json:"id,omitempty" yaml:"id,omitempty"`
	Alias                  string                   `json:"alias,omitempty" yaml:"alias,omitempty"`
	RequiredAttributes     []string                 `json:"requiredAttributes,omitempty" yaml:"requiredAttributes,omitempty"`
	RemoteCollectionPath   string                   `json:"remoteCollectionPath,omitempty" yaml:"remoteCollectionPath,omitempty"`
	Format                 string                   `json:"format,omitempty" yaml:"format,omitempty"`
	Defaults               *DefaultsSpec            `json:"defaults,omitempty" yaml:"defaults,omitempty"`
	Secret                 *bool                    `json:"secret,omitempty" yaml:"secret,omitempty"`
	SecretAttributes       []string                 `json:"secretAttributes,omitempty" yaml:"secretAttributes,omitempty"`
	ExternalizedAttributes []ExternalizedAttribute  `json:"externalizedAttributes,omitempty" yaml:"externalizedAttributes,omitempty"`
	Operations             map[string]OperationSpec `json:"operations,omitempty" yaml:"operations,omitempty"`
	Transforms             []TransformStep          `json:"transforms,omitempty" yaml:"transforms,omitempty"`
}

func (m ResourceMetadata) IsWholeResourceSecret() bool {
	return m.Secret != nil && *m.Secret
}

type ExternalizedAttribute struct {
	Path           string `json:"path,omitempty" yaml:"path,omitempty"`
	File           string `json:"file,omitempty" yaml:"file,omitempty"`
	Template       string `json:"template,omitempty" yaml:"template,omitempty"`
	Mode           string `json:"mode,omitempty" yaml:"mode,omitempty"`
	SaveBehavior   string `json:"saveBehavior,omitempty" yaml:"saveBehavior,omitempty"`
	RenderBehavior string `json:"renderBehavior,omitempty" yaml:"renderBehavior,omitempty"`
	Enabled        *bool  `json:"enabled,omitempty" yaml:"enabled,omitempty"`
}

type ResolvedExternalizedAttribute struct {
	Path           string
	File           string
	Template       string
	Mode           string
	SaveBehavior   string
	RenderBehavior string
	Enabled        bool
}

type OperationSpec struct {
	Method      string            `json:"method,omitempty" yaml:"method,omitempty"`
	Path        string            `json:"path,omitempty" yaml:"path,omitempty"`
	Query       map[string]string `json:"query,omitempty" yaml:"query,omitempty"`
	Headers     map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Accept      string            `json:"accept,omitempty" yaml:"accept,omitempty"`
	ContentType string            `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Body        any               `json:"body,omitempty" yaml:"body,omitempty"`
	Transforms  []TransformStep   `json:"transforms,omitempty" yaml:"transforms,omitempty"`
	Validate    *OperationValidationSpec
}

type TransformStep struct {
	SelectAttributes  []string `json:"selectAttributes,omitempty" yaml:"selectAttributes,omitempty"`
	ExcludeAttributes []string `json:"excludeAttributes,omitempty" yaml:"excludeAttributes,omitempty"`
	JQExpression      string   `json:"jqExpression,omitempty" yaml:"jqExpression,omitempty"`
}

type OperationValidationSpec struct {
	RequiredAttributes []string              `json:"requiredAttributes,omitempty" yaml:"requiredAttributes,omitempty"`
	Assertions         []ValidationAssertion `json:"assertions,omitempty" yaml:"assertions,omitempty"`
	SchemaRef          string                `json:"schemaRef,omitempty" yaml:"schemaRef,omitempty"`
}

type ValidationAssertion struct {
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
	JQ      string `json:"jq,omitempty" yaml:"jq,omitempty"`
}

func (o Operation) IsValid() bool {
	switch o {
	case OperationGet, OperationCreate, OperationUpdate, OperationDelete, OperationList, OperationCompare:
		return true
	default:
		return false
	}
}
