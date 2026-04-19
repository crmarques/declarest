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

package identity

import (
	"fmt"
	"path"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/metadata/identitytemplate"
	"github.com/crmarques/declarest/resource"
)

const defaultIdentityPointer = "/id"

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

	if resolved, ok, err := resolveIdentityField("resource.alias", md.Alias, payload); err != nil {
		return "", "", err
	} else if ok {
		alias = resolved
	}

	if resolved, ok, err := resolveIdentityField("resource.id", md.ID, payload); err != nil {
		return "", "", err
	} else if ok {
		remoteID = resolved
	}

	if strings.TrimSpace(alias) == "" {
		alias = aliasForLogicalPath(logicalPath)
	}
	if strings.TrimSpace(remoteID) == "" {
		remoteID = alias
	}

	if err := resource.ValidateLogicalPathSegment(alias); err != nil {
		return "", "", faults.Invalid(
			fmt.Sprintf("resource.alias rendered invalid logical path segment %q", alias),
			err,
		)
	}
	if err := resource.ValidateLogicalPathSegment(remoteID); err != nil {
		return "", "", faults.Invalid(
			fmt.Sprintf("resource.id rendered invalid logical path segment %q", remoteID),
			err,
		)
	}

	return alias, remoteID, nil
}

func ResolveAliasAndRemoteIDForListItem(payload map[string]any, md metadata.ResourceMetadata) (string, string, error) {
	var alias string
	if resolved, ok, err := resolveIdentityField("resource.alias", md.Alias, payload); err != nil {
		return "", "", err
	} else if ok {
		alias = resolved
	}
	if alias == "" {
		if resolved, ok, err := resolveIdentityField("resource.id", md.ID, payload); err != nil {
			return "", "", err
		} else if ok {
			alias = resolved
		}
	}
	if alias == "" {
		return "", "", faults.NewTypedError(
			faults.ValidationError,
			"list item alias could not be resolved from metadata alias/id templates",
			nil,
		)
	}

	remoteID := alias
	if resolved, ok, err := resolveIdentityField("resource.id", md.ID, payload); err != nil {
		return "", "", err
	} else if ok {
		remoteID = resolved
	}

	return alias, remoteID, nil
}

// resolveIdentityField resolves an identity field (alias or id) from a raw
// template string against the given payload. It returns the resolved value,
// whether a value was found, and any error. The template is first checked as
// a simple JSON pointer; if not, it is rendered as a full identity template.
func resolveIdentityField(field string, rawTemplate string, payload any) (string, bool, error) {
	template := effectiveIdentityTemplate(rawTemplate)

	if pointer, ok, err := simpleIdentityPointer(field, template); err != nil {
		return "", false, err
	} else if ok {
		value, found, lookupErr := lookupSimplePointer(payload, pointer)
		if lookupErr != nil {
			return "", false, faults.Invalid(field+" must resolve from payload data", lookupErr)
		}
		if found {
			return value, true, nil
		}
		return "", false, nil
	}

	rendered, renderErr := renderIdentityTemplate(field, template, payload)
	if renderErr != nil {
		return "", false, renderErr
	}
	return rendered, true, nil
}

func RequiredAttributes(md metadata.ResourceMetadata) ([]string, error) {
	attributes := append([]string(nil), md.RequiredAttributes...)
	addPointer := orderedStringCollector(&attributes)

	if template := strings.TrimSpace(md.Alias); template != "" {
		pointers, err := identitytemplate.ExtractPointers(template)
		if err != nil {
			return nil, faults.Invalid("resource.alias must be a valid identity template", err)
		}
		for _, pointer := range pointers {
			addPointer(pointer)
		}
	}
	if template := strings.TrimSpace(md.ID); template != "" {
		pointers, err := identitytemplate.ExtractPointers(template)
		if err != nil {
			return nil, faults.Invalid("resource.id must be a valid identity template", err)
		}
		for _, pointer := range pointers {
			addPointer(pointer)
		}
	}

	return attributes, nil
}

func SimpleAliasPointer(md metadata.ResourceMetadata) (string, bool, error) {
	return simpleIdentityPointer("resource.alias", md.Alias)
}

func SimpleIDPointer(md metadata.ResourceMetadata) (string, bool, error) {
	return simpleIdentityPointer("resource.id", md.ID)
}

func aliasForLogicalPath(logicalPath string) string {
	trimmed := strings.TrimSpace(logicalPath)
	if trimmed == "" || trimmed == "/" {
		return "/"
	}
	return path.Base(trimmed)
}

func renderIdentityTemplate(field string, raw string, payload any) (string, error) {
	rendered, err := identitytemplate.Render(raw, payload)
	if err != nil {
		return "", faults.Invalid(field+" must resolve from payload data", err)
	}
	trimmed := strings.TrimSpace(rendered)
	if trimmed == "" {
		return "", faults.Invalid(field+" must not resolve to an empty value", nil)
	}
	return trimmed, nil
}

func simpleIdentityPointer(field string, raw string) (string, bool, error) {
	template := strings.TrimSpace(raw)
	if template == "" {
		return "", false, nil
	}
	pointer, ok, err := identitytemplate.SimplePointer(template)
	if err != nil {
		return "", false, faults.Invalid(field+" must be a valid identity template", err)
	}
	return pointer, ok, nil
}

func lookupSimplePointer(payload any, pointer string) (string, bool, error) {
	value, found, err := resource.LookupJSONPointerString(payload, pointer)
	if err != nil {
		return "", false, err
	}
	if !found {
		return "", false, nil
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false, nil
	}
	return trimmed, true, nil
}

func effectiveIdentityTemplate(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultIdentityPointer
	}
	return trimmed
}

func orderedStringCollector(target *[]string) func(string) {
	seen := make(map[string]struct{}, len(*target))
	for _, item := range *target {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		seen[trimmed] = struct{}{}
	}

	return func(value string) {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return
		}
		if _, exists := seen[trimmed]; exists {
			return
		}
		seen[trimmed] = struct{}{}
		*target = append(*target, trimmed)
	}
}
