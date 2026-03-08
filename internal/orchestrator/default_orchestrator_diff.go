package orchestrator

import (
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
	"github.com/itchyny/gojq"
)

func applyCompareTransforms(value resource.Value, operationSpec metadata.OperationSpec) (resource.Value, error) {
	steps := metadata.OrderedPayloadMutationSteps(operationSpec)
	if len(steps) == 0 {
		return resource.Normalize(value)
	}

	current, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	for _, step := range steps {
		switch metadata.PayloadMutationStepType(step) {
		case "selectAttributes":
			current, err = applyFilterPointers(current, step.SelectAttributes)
		case "suppressAttributes":
			current, err = applySuppressPointers(current, step.SuppressAttributes)
		case "jqExpression":
			current, err = applyCompareJQ(current, step.JQExpression)
		}
		if err != nil {
			return nil, err
		}
	}

	return resource.Normalize(current)
}

func applyCompareJQ(value resource.Value, expression string) (resource.Value, error) {
	trimmedExpression := strings.TrimSpace(expression)
	if trimmedExpression == "" {
		return value, nil
	}

	query, err := gojq.Parse(trimmedExpression)
	if err != nil {
		return nil, faults.NewValidationError("invalid compare jq expression", err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return nil, faults.NewValidationError("invalid compare jq expression", err)
	}

	iterator := code.Run(value)
	results := make([]any, 0, 1)
	for {
		item, ok := iterator.Next()
		if !ok {
			break
		}
		if itemErr, isErr := item.(error); isErr {
			return nil, faults.NewValidationError("failed to evaluate compare jq expression", itemErr)
		}
		results = append(results, item)
	}

	switch len(results) {
	case 0:
		return nil, nil
	case 1:
		return resource.Normalize(results[0])
	default:
		return resource.Normalize(results)
	}
}

func buildDiffEntries(logicalPath string, local resource.Value, remote resource.Value) []resource.DiffEntry {
	entries := make([]resource.DiffEntry, 0)
	collectDiffEntries(&entries, logicalPath, "", local, remote)
	return entries
}

func collectDiffEntries(entries *[]resource.DiffEntry, logicalPath string, pointer string, local any, remote any) {
	if reflect.DeepEqual(local, remote) {
		return
	}
	if resource.IsBinaryValue(local) || resource.IsBinaryValue(remote) {
		appendDiffEntry(entries, logicalPath, "", "replace", local, remote)
		return
	}

	localObject, localIsObject := local.(map[string]any)
	remoteObject, remoteIsObject := remote.(map[string]any)
	if localIsObject && remoteIsObject {
		keys := make([]string, 0, len(localObject)+len(remoteObject))
		seen := make(map[string]struct{}, len(localObject)+len(remoteObject))
		for key := range localObject {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
		for key := range remoteObject {
			if _, found := seen[key]; found {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			nextPointer := pointer + "/" + resource.EscapeJSONPointerToken(key)
			localValue, localFound := localObject[key]
			remoteValue, remoteFound := remoteObject[key]

			switch {
			case !localFound:
				appendDiffEntry(entries, logicalPath, nextPointer, "add", nil, remoteValue)
			case !remoteFound:
				appendDiffEntry(entries, logicalPath, nextPointer, "remove", localValue, nil)
			default:
				collectDiffEntries(entries, logicalPath, nextPointer, localValue, remoteValue)
			}
		}
		return
	}

	localArray, localIsArray := local.([]any)
	remoteArray, remoteIsArray := remote.([]any)
	if localIsArray && remoteIsArray {
		maxLength := len(localArray)
		if len(remoteArray) > maxLength {
			maxLength = len(remoteArray)
		}

		for idx := range maxLength {
			nextPointer := pointer + "/" + strconv.Itoa(idx)

			switch {
			case idx >= len(localArray):
				appendDiffEntry(entries, logicalPath, nextPointer, "add", nil, remoteArray[idx])
			case idx >= len(remoteArray):
				appendDiffEntry(entries, logicalPath, nextPointer, "remove", localArray[idx], nil)
			default:
				collectDiffEntries(entries, logicalPath, nextPointer, localArray[idx], remoteArray[idx])
			}
		}
		return
	}

	appendDiffEntry(entries, logicalPath, pointer, "replace", local, remote)
}

func appendDiffEntry(
	entries *[]resource.DiffEntry,
	logicalPath string,
	pointer string,
	operation string,
	local any,
	remote any,
) {
	*entries = append(*entries, resource.DiffEntry{
		ResourcePath: logicalPath,
		Path:         pointer,
		Operation:    operation,
		Local:        local,
		Remote:       remote,
	})
}

