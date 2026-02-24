package metadata

import "net/http"

var defaultMetadataOperations = []Operation{
	OperationCreate,
	OperationList,
	OperationGet,
	OperationUpdate,
	OperationDelete,
	OperationCompare,
}

// DefaultResourceMetadata returns the base metadata payload used as the
// foundation for merging repository overrides. The returned metadata already
// includes the canonical operation entries so downstream callers can render
// each operation even when no overrides exist.
func DefaultResourceMetadata() ResourceMetadata {
	operations := make(map[string]OperationSpec, len(defaultMetadataOperations))
	for _, operation := range defaultMetadataOperations {
		defaultAccept := "application/{{resource_format .}}"
		defaultContentType := ""
		if operation == OperationCreate || operation == OperationUpdate {
			defaultContentType = "application/{{resource_format .}}"
		}
		operations[string(operation)] = OperationSpec{
			Method:      DefaultOperationMethod(operation),
			Path:        defaultOperationPathTemplate(operation),
			Query:       map[string]string{},
			Headers:     map[string]string{},
			Accept:      defaultAccept,
			ContentType: defaultContentType,
			Filter:      nil,
			Suppress:    []string{},
		}
	}

	return ResourceMetadata{
		Operations: operations,
		Filter:     nil,
		Suppress:   []string{},
	}
}

// DefaultOperationMethod returns the HTTP method associated with the provided
// metadata operation when no explicit override is provided.
func DefaultOperationMethod(operation Operation) string {
	switch operation {
	case OperationCreate:
		return http.MethodPost
	case OperationUpdate:
		return http.MethodPut
	case OperationDelete:
		return http.MethodDelete
	case OperationGet, OperationList, OperationCompare:
		return http.MethodGet
	default:
		return http.MethodGet
	}
}
