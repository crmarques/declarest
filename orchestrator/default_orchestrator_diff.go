package orchestrator

import (
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func applyCompareTransforms(value resource.Value, operationSpec metadata.OperationSpec) (resource.Value, error) {
	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	filtered := normalized
	if len(operationSpec.Filter) > 0 {
		filtered, err = applyFilterPointers(filtered, operationSpec.Filter)
		if err != nil {
			return nil, err
		}
	}

	if len(operationSpec.Suppress) == 0 {
		return filtered, nil
	}

	return applySuppressPointers(filtered, operationSpec.Suppress)
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
			nextPointer := pointer + "/" + escapePointerToken(key)
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

func escapePointerToken(value string) string {
	escaped := strings.ReplaceAll(value, "~", "~0")
	return strings.ReplaceAll(escaped, "/", "~1")
}
