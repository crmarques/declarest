package openapi

import (
	"strings"

	"declarest/internal/resource"
)

const (
	defaultResourcePathTemplate   = "./{{.id}}"
	defaultCollectionPathTemplate = "."
)

func ApplyDefaults(meta resource.ResourceMetadata, resourcePath string, isCollection bool, spec *Spec) resource.ResourceMetadata {
	if spec == nil {
		return meta
	}
	if meta.OperationInfo == nil || meta.ResourceInfo == nil {
		return meta
	}

	collectionPath := strings.TrimSpace(meta.ResourceInfo.CollectionPath)
	if collectionPath == "" {
		return meta
	}

	collectionItem := spec.MatchPath(collectionPath)
	resourceItem := spec.MatchPath(resourcePath)

	if meta.OperationInfo.ListCollection != nil {
		if op := collectionItem.Operation("get"); op != nil {
			applyOperation(meta.OperationInfo.ListCollection, op)
		}
	}

	if isCollection {
		return meta
	}

	if meta.OperationInfo.GetResource != nil {
		if op := resourceItem.Operation("get"); op != nil {
			applyOperation(meta.OperationInfo.GetResource, op)
		}
	}

	if meta.OperationInfo.DeleteResource != nil {
		if op := resourceItem.Operation("delete"); op != nil {
			applyOperation(meta.OperationInfo.DeleteResource, op)
		}
	}

	if meta.OperationInfo.CreateResource != nil {
		if op, useResource := selectCreateOperation(collectionItem, resourceItem); op != nil {
			applyOperation(meta.OperationInfo.CreateResource, op)
			if meta.OperationInfo.CreateResource.URL != nil {
				meta.OperationInfo.CreateResource.URL.Path = createPathTemplate(useResource)
			}
		}
	}

	if meta.OperationInfo.UpdateResource != nil {
		if op := selectUpdateOperation(resourceItem); op != nil {
			applyOperation(meta.OperationInfo.UpdateResource, op)
		}
	}

	return meta
}

func selectCreateOperation(collectionItem, resourceItem *PathItem) (*Operation, bool) {
	if collectionItem != nil {
		if op := collectionItem.Operation("post"); op != nil {
			return op, false
		}
	}
	if resourceItem != nil {
		if op := resourceItem.Operation("put"); op != nil {
			return op, true
		}
		if op := resourceItem.Operation("post"); op != nil {
			return op, true
		}
		if op := resourceItem.Operation("patch"); op != nil {
			return op, true
		}
	}
	return nil, false
}

func selectUpdateOperation(resourceItem *PathItem) *Operation {
	if resourceItem == nil {
		return nil
	}
	if op := resourceItem.Operation("put"); op != nil {
		return op
	}
	if op := resourceItem.Operation("patch"); op != nil {
		return op
	}
	if op := resourceItem.Operation("post"); op != nil {
		return op
	}
	return nil
}

func applyOperation(target *resource.OperationMetadata, op *Operation) {
	if target == nil || op == nil {
		return
	}
	target.HTTPMethod = strings.ToUpper(op.Method)
	headers := resource.HeaderMap(target.HTTPHeaders)

	if len(op.ResponseContentTypes) > 0 {
		headers["Accept"] = []string{pickContentType(op.ResponseContentTypes)}
	}
	if len(op.RequestContentTypes) > 0 && resource.MethodSupportsBody(target.HTTPMethod) {
		headers["Content-Type"] = []string{pickContentType(op.RequestContentTypes)}
	}

	target.HTTPHeaders = resource.HeaderListFromMap(headers)
}

func pickContentType(options []string) string {
	for _, option := range options {
		if strings.EqualFold(option, "application/json") {
			return option
		}
	}
	for _, option := range options {
		if strings.Contains(strings.ToLower(option), "+json") {
			return option
		}
	}
	if len(options) == 0 {
		return ""
	}
	return options[0]
}

func createPathTemplate(useResource bool) string {
	if useResource {
		return defaultResourcePathTemplate
	}
	return defaultCollectionPathTemplate
}
