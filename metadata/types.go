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
	SecretsFromAttributes []string                 `json:"secretsFromAttributes" yaml:"secretsFromAttributes"`
	Operations            map[string]OperationSpec `json:"operations" yaml:"operations"`
	Filter                []string                 `json:"filter" yaml:"filter"`
	Suppress              []string                 `json:"suppress" yaml:"suppress"`
	JQ                    string                   `json:"jq,omitempty" yaml:"jq,omitempty"`
}

type OperationSpec struct {
	Method      string            `json:"method,omitempty" yaml:"method,omitempty"`
	Path        string            `json:"path,omitempty" yaml:"path,omitempty"`
	Query       map[string]string `json:"query" yaml:"query"`
	Headers     map[string]string `json:"headers" yaml:"headers"`
	Accept      string            `json:"accept,omitempty" yaml:"accept,omitempty"`
	ContentType string            `json:"contentType,omitempty" yaml:"contentType,omitempty"`
	Body        any               `json:"body,omitempty" yaml:"body,omitempty"`
	Filter      []string          `json:"filter" yaml:"filter"`
	Suppress    []string          `json:"suppress" yaml:"suppress"`
	JQ          string            `json:"jq,omitempty" yaml:"jq,omitempty"`
}

func (o Operation) IsValid() bool {
	switch o {
	case OperationGet, OperationCreate, OperationUpdate, OperationDelete, OperationList, OperationCompare:
		return true
	default:
		return false
	}
}
