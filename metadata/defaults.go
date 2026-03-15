// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metadata

import (
	"net/http"

	"github.com/crmarques/declarest/resource"
)

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
		operations[string(operation)] = OperationSpec{
			Method:     DefaultOperationMethod(operation),
			Path:       defaultOperationPathTemplate(operation),
			Query:      map[string]string{},
			Headers:    map[string]string{},
			Transforms: nil,
		}
	}

	return ResourceMetadata{
		ID:         resource.JSONPointerForObjectKey("id"),
		Alias:      resource.JSONPointerForObjectKey("id"),
		Operations: operations,
		Transforms: nil,
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
