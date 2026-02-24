package identity

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func LookupScalarAttribute(payload map[string]any, attribute string) (string, bool) {
	trimmed := strings.TrimSpace(attribute)
	if trimmed == "" {
		return "", false
	}

	current := any(payload)
	for _, segment := range strings.Split(trimmed, ".") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return "", false
		}

		mapValue, ok := current.(map[string]any)
		if !ok {
			return "", false
		}

		next, exists := mapValue[segment]
		if !exists {
			return "", false
		}
		current = next
	}

	return scalarString(current)
}

func ResolveAliasAndRemoteID(logicalPath string, md metadata.ResourceMetadata, payload resource.Value) (string, string, error) {
	alias := aliasForLogicalPath(logicalPath)
	remoteID := alias

	payloadMap, ok := payload.(map[string]any)
	if !ok {
		return alias, remoteID, nil
	}

	if attr := strings.TrimSpace(md.AliasFromAttribute); attr != "" {
		if value, found := LookupScalarAttribute(payloadMap, attr); found && strings.TrimSpace(value) != "" {
			alias = strings.TrimSpace(value)
		}
	}

	if attr := strings.TrimSpace(md.IDFromAttribute); attr != "" {
		if value, found := LookupScalarAttribute(payloadMap, attr); found && strings.TrimSpace(value) != "" {
			remoteID = strings.TrimSpace(value)
		}
	} else {
		remoteID = alias
	}

	if strings.TrimSpace(alias) == "" {
		alias = aliasForLogicalPath(logicalPath)
	}
	if strings.TrimSpace(remoteID) == "" {
		remoteID = alias
	}

	return alias, remoteID, nil
}

func ResolveAliasAndRemoteIDForListItem(payload map[string]any, md metadata.ResourceMetadata) (string, string, error) {
	var alias string
	if attr := strings.TrimSpace(md.AliasFromAttribute); attr != "" {
		alias, _ = LookupScalarAttribute(payload, attr)
	}
	if alias == "" {
		if attr := strings.TrimSpace(md.IDFromAttribute); attr != "" {
			alias, _ = LookupScalarAttribute(payload, attr)
		}
	}
	if alias == "" {
		return "", "", faults.NewTypedError(
			faults.ValidationError,
			"list item alias could not be resolved from metadata attributes",
			nil,
		)
	}

	remoteID := alias
	if attr := strings.TrimSpace(md.IDFromAttribute); attr != "" {
		if value, ok := LookupScalarAttribute(payload, attr); ok && strings.TrimSpace(value) != "" {
			remoteID = strings.TrimSpace(value)
		}
	}

	return alias, remoteID, nil
}

func aliasForLogicalPath(logicalPath string) string {
	trimmed := strings.TrimSpace(logicalPath)
	if trimmed == "" || trimmed == "/" {
		return "/"
	}
	return path.Base(trimmed)
}

func scalarString(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return typed, typed != ""
	case fmt.Stringer:
		text := strings.TrimSpace(typed.String())
		return text, text != ""
	case jsonNumberLike:
		text := strings.TrimSpace(typed.String())
		return text, text != ""
	case int:
		return strconv.Itoa(typed), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typed), true
	default:
		return "", false
	}
}

type jsonNumberLike interface {
	String() string
}
