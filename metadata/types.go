package metadata

import "github.com/crmarques/declarest/core"

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
	Body        core.Resource
}
