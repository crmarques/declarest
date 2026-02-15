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
	IDFromAttribute    string
	AliasFromAttribute string
	Operations         map[string]OperationSpec
	Filter             []string
	Suppress           []string
	JQ                 string
}

type OperationSpec struct {
	Method      string
	Path        string
	Query       map[string]string
	Headers     map[string]string
	Accept      string
	ContentType string
	Body        any
}
