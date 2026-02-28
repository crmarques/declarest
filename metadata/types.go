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

type ResourceMetadata struct {
	IDFromAttribute       string                   `json:"idFromAttribute,omitempty" yaml:"idFromAttribute,omitempty"`
	AliasFromAttribute    string                   `json:"aliasFromAttribute,omitempty" yaml:"aliasFromAttribute,omitempty"`
	CollectionPath        string                   `json:"collectionPath,omitempty" yaml:"collectionPath,omitempty"`
	SecretsFromAttributes []string                 `json:"secretsFromAttributes,omitempty" yaml:"secretsFromAttributes,omitempty"`
	Operations            map[string]OperationSpec `json:"operations,omitempty" yaml:"operations,omitempty"`
	Filter                []string                 `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress              []string                 `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ                    string                   `json:"jq,omitempty" yaml:"jq,omitempty"`
	PayloadTransformOrder []string                 `json:"-" yaml:"-"`
}

type OperationSpec struct {
	Method                string            `json:"method,omitempty" yaml:"method,omitempty"`
	Path                  string            `json:"path,omitempty" yaml:"path,omitempty"`
	Query                 map[string]string `json:"query,omitempty" yaml:"query,omitempty"`
	Headers               map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`
	Accept                string            `json:"accept,omitempty" yaml:"accept,omitempty"`
	ContentType           string            `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Body                  any               `json:"body,omitempty" yaml:"body,omitempty"`
	Filter                []string          `json:"filter,omitempty" yaml:"filter,omitempty"`
	Suppress              []string          `json:"suppress,omitempty" yaml:"suppress,omitempty"`
	JQ                    string            `json:"jq,omitempty" yaml:"jq,omitempty"`
	Validate              *OperationValidationSpec
	PayloadTransformOrder []string `json:"-" yaml:"-"`
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
