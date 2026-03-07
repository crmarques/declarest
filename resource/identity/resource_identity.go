package identity

import (
	"path"
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
	value, found, err := resource.LookupJSONPointerString(payload, trimmed)
	if err != nil {
		return "", false
	}
	return value, found
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
