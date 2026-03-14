package metadata

import (
	"path"
	"sort"

	"github.com/crmarques/declarest/faults"
)

func parseInferTarget(logicalPath string) (inferTarget, error) {
	pathDescriptor, err := ParsePathDescriptor(logicalPath)
	if err != nil {
		return inferTarget{}, err
	}

	if !pathDescriptor.Collection && pathDescriptor.Selector == "/" {
		return inferTarget{}, faults.NewTypedError(
			faults.ValidationError,
			"resource metadata path must not target root",
			nil,
		)
	}

	return inferTarget{
		Selector:   pathDescriptor.Selector,
		Segments:   cloneStringSlice(pathDescriptor.Segments),
		Collection: pathDescriptor.Collection,
	}, nil
}

func inferFallbackMetadata(target inferTarget) ResourceMetadata {
	if !target.Collection {
		collectionPath := path.Dir(target.Selector)
		if collectionPath == "." || collectionPath == "" {
			collectionPath = "/"
		}

		return ResourceMetadata{
			RemoteCollectionPath: collectionPath,
			Operations: map[string]OperationSpec{
				string(OperationGet): {
					Method: "GET",
					Path:   target.Selector,
				},
				string(OperationCreate): {
					Method: "POST",
					Path:   target.Selector,
				},
				string(OperationUpdate): {
					Method: "PUT",
					Path:   target.Selector,
				},
				string(OperationDelete): {
					Method: "DELETE",
					Path:   target.Selector,
				},
				string(OperationList): {
					Method: "GET",
					Path:   collectionPath,
				},
				string(OperationCompare): {
					Method: "GET",
					Path:   target.Selector,
				},
			},
		}
	}

	idFieldName, _ := inferIdentityAttributes(target, "", nil)
	collectionPath, resourcePath := inferCollectionAndResourceTemplatePaths(target, idFieldName)
	operations := make(map[string]OperationSpec)
	if collectionPath != "" {
		operations[string(OperationList)] = OperationSpec{Method: "GET", Path: collectionPath}
		operations[string(OperationCreate)] = OperationSpec{Method: "POST", Path: collectionPath}
	}
	if resourcePath != "" {
		operations[string(OperationGet)] = OperationSpec{Method: "GET", Path: resourcePath}
		operations[string(OperationUpdate)] = OperationSpec{Method: "PUT", Path: resourcePath}
		operations[string(OperationDelete)] = OperationSpec{Method: "DELETE", Path: resourcePath}
		operations[string(OperationCompare)] = OperationSpec{Method: "GET", Path: resourcePath}
	}

	return ResourceMetadata{
		RemoteCollectionPath: collectionPath,
		Operations:           operations,
	}
}

func inferMetadataFromOpenAPISpec(
	target inferTarget,
	openAPISpec any,
) (ResourceMetadata, string, map[string]struct{}) {
	pathDefinitions := openAPIPathDefinitions(openAPISpec)
	if len(pathDefinitions) == 0 {
		return ResourceMetadata{}, "", nil
	}
	defaults := inferFallbackMetadata(target)
	pathItems := openAPIPathItems(openAPISpec)

	var collectionCandidate openAPICandidate
	var resourceCandidate openAPICandidate

	if target.Collection {
		collectionCandidate = selectOpenAPICandidate(target.Segments, len(target.Segments), pathDefinitions)
		resourceCandidate = selectOpenAPICandidate(target.Segments, len(target.Segments)+1, pathDefinitions)
	} else {
		resourceCandidate = selectOpenAPICandidate(target.Segments, len(target.Segments), pathDefinitions)
		collectionCandidate = selectOpenAPICandidate(splitPathSegments(path.Dir(target.Selector)), len(target.Segments)-1, pathDefinitions)
	}

	operations := make(map[string]OperationSpec)
	collectionPath := ""
	if collectionCandidate.path != "" {
		defaultCollectionPath := defaults.Operations[string(OperationList)].Path
		metadataCollectionPath := openAPIPathToMetadataTemplate(collectionCandidate.path, defaultCollectionPath)
		collectionPath = metadataCollectionPath
		if hasOpenAPIMethod(collectionCandidate.methods, "get") {
			operations[string(OperationList)] = OperationSpec{
				Method: "GET",
				Path:   metadataCollectionPath,
			}
		}
		if hasOpenAPIMethod(collectionCandidate.methods, "post") {
			createValidation := inferOpenAPIOperationValidationSpec(
				collectionCandidate,
				"post",
				pathItems,
				openAPISpec,
			)
			operations[string(OperationCreate)] = OperationSpec{
				Method:   "POST",
				Path:     metadataCollectionPath,
				Validate: createValidation,
			}
		}
	}

	resourceIdentityAttribute := ""
	resourceSchemaAttributes := inferOpenAPIResponseAttributes(resourceCandidate, pathItems, openAPISpec)
	if len(resourceSchemaAttributes) == 0 {
		resourceSchemaAttributes = inferOpenAPIResponseAttributes(collectionCandidate, pathItems, openAPISpec)
	}
	if resourceCandidate.path != "" {
		defaultResourcePath := defaults.Operations[string(OperationGet)].Path
		metadataResourcePath := openAPIPathToMetadataTemplate(resourceCandidate.path, defaultResourcePath)
		resourceIdentityAttribute, _ = lastOpenAPIVariable(resourceCandidate.segments)

		if hasOpenAPIMethod(resourceCandidate.methods, "get") {
			operations[string(OperationGet)] = OperationSpec{
				Method: "GET",
				Path:   metadataResourcePath,
			}
			operations[string(OperationCompare)] = OperationSpec{
				Method: "GET",
				Path:   metadataResourcePath,
			}
		}
		if hasOpenAPIMethod(resourceCandidate.methods, "put") {
			updateValidation := inferOpenAPIOperationValidationSpec(
				resourceCandidate,
				"put",
				pathItems,
				openAPISpec,
			)
			operations[string(OperationUpdate)] = OperationSpec{
				Method:   "PUT",
				Path:     metadataResourcePath,
				Validate: updateValidation,
			}
		} else if hasOpenAPIMethod(resourceCandidate.methods, "patch") {
			updateValidation := inferOpenAPIOperationValidationSpec(
				resourceCandidate,
				"patch",
				pathItems,
				openAPISpec,
			)
			operations[string(OperationUpdate)] = OperationSpec{
				Method:   "PATCH",
				Path:     metadataResourcePath,
				Validate: updateValidation,
			}
		}
		if hasOpenAPIMethod(resourceCandidate.methods, "delete") {
			operations[string(OperationDelete)] = OperationSpec{
				Method: "DELETE",
				Path:   metadataResourcePath,
			}
		}
	}

	if len(operations) == 0 {
		return ResourceMetadata{}, resourceIdentityAttribute, resourceSchemaAttributes
	}
	payloadTypeCandidates := inferOpenAPIMetadataPayloadTypes(collectionCandidate, resourceCandidate, pathItems, openAPISpec)
	return ResourceMetadata{
		RemoteCollectionPath: collectionPath,
		Operations:           operations,
		Format:               inferredOpenAPIMetadataFormat(payloadTypeCandidates),
	}, resourceIdentityAttribute, resourceSchemaAttributes
}

func inferOpenAPIMetadataPayloadTypes(
	collectionCandidate openAPICandidate,
	resourceCandidate openAPICandidate,
	pathItems map[string]map[string]any,
	openAPISpec any,
) []string {
	seen := map[string]struct{}{}
	collectOpenAPICandidatePayloadTypes(seen, collectionCandidate, pathItems, openAPISpec)
	collectOpenAPICandidatePayloadTypes(seen, resourceCandidate, pathItems, openAPISpec)
	if len(seen) == 0 {
		return nil
	}

	payloadTypes := make([]string, 0, len(seen))
	for payloadType := range seen {
		payloadTypes = append(payloadTypes, payloadType)
	}
	sort.Strings(payloadTypes)
	return payloadTypes
}

func inferredOpenAPIMetadataFormat(payloadTypes []string) string {
	switch len(payloadTypes) {
	case 0:
		return ""
	case 1:
		return payloadTypes[0]
	default:
		return ResourceFormatAny
	}
}

func collectOpenAPICandidatePayloadTypes(
	target map[string]struct{},
	candidate openAPICandidate,
	pathItems map[string]map[string]any,
	openAPISpec any,
) {
	if candidate.path == "" || len(pathItems) == 0 {
		return
	}

	pathItem, found := pathItems[candidate.path]
	if !found {
		return
	}

	methods := make([]string, 0, len(candidate.methods))
	for method := range candidate.methods {
		methods = append(methods, method)
	}
	sort.Strings(methods)
	for _, method := range methods {
		operationValue, found := pathItem[method]
		if !found {
			continue
		}
		operationItem, ok := asStringMap(operationValue)
		if !ok {
			continue
		}
		for _, payloadType := range inferOpenAPIOperationPayloadTypes(operationItem, openAPISpec) {
			target[payloadType] = struct{}{}
		}
	}
}
