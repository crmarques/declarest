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
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

const (
	DefaultsModeInherit = "inherit"
	DefaultsModeIgnore  = "ignore"
	DefaultsModeReplace = "replace"
)

type DefaultsSpec struct {
	Mode        string         `json:"mode,omitempty" yaml:"mode,omitempty"`
	UseProfiles []string       `json:"useProfiles,omitempty" yaml:"useProfiles,omitempty"`
	Value       any            `json:"value,omitempty" yaml:"value,omitempty"`
	Profiles    map[string]any `json:"profiles,omitempty" yaml:"profiles,omitempty"`
}

var (
	defaultsIncludePattern = regexp.MustCompile(`^\{\{include ([^{}\s]+)\}\}$`)
	profileNamePattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

func SupportedDefaultsArtifactExtensions() []string {
	return []string{".json", ".properties", ".yaml", ".yml"}
}

func DefaultsSupportsFileBackedDescriptor(descriptor resource.PayloadDescriptor) bool {
	switch resource.NormalizePayloadDescriptor(descriptor).Extension {
	case ".json", ".properties", ".yaml", ".yml":
		return true
	default:
		return false
	}
}

func DefaultsSupportsFileBackedPayloadType(payloadType string) bool {
	switch resource.NormalizePayloadType(payloadType) {
	case resource.PayloadTypeJSON, resource.PayloadTypeProperties, resource.PayloadTypeYAML:
		return true
	default:
		return false
	}
}

func DefaultsFileName(profile string, descriptor resource.PayloadDescriptor) (string, error) {
	resolved := resource.NormalizePayloadDescriptor(descriptor)
	if !DefaultsSupportsFileBackedDescriptor(resolved) {
		return "", faults.Invalid(
			fmt.Sprintf(
				"resource defaults files support only json, yaml, yml, or properties; got %q",
				resolved.Extension,
			),
			nil,
		)
	}

	if strings.TrimSpace(profile) == "" {
		return "defaults" + resolved.Extension, nil
	}
	if err := ValidateDefaultsProfileName(profile); err != nil {
		return "", err
	}
	return "defaults-" + profile + resolved.Extension, nil
}

func DefaultsIncludePlaceholder(file string) string {
	return "{{include " + strings.TrimSpace(file) + "}}"
}

func ParseDefaultsIncludeReference(value string) (string, bool) {
	matches := defaultsIncludePattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 2 {
		return "", false
	}
	return matches[1], true
}

func ValidateDefaultsProfileName(value string) error {
	trimmed := strings.TrimSpace(value)
	if !profileNamePattern.MatchString(trimmed) {
		return faults.Invalid(
			fmt.Sprintf("resource.defaults profile %q is invalid", value),
			nil,
		)
	}
	return nil
}

func ValidateDefaultsSpec(value *DefaultsSpec) error {
	if value == nil {
		return nil
	}
	if _, err := ValidateDefaultsMode(value.Mode); err != nil {
		return err
	}
	for idx, profile := range value.UseProfiles {
		if strings.TrimSpace(profile) == "" {
			return faults.Invalid(
				fmt.Sprintf("resource.defaults.useProfiles[%d] must not be empty", idx),
				nil,
			)
		}
		if err := ValidateDefaultsProfileName(profile); err != nil {
			return err
		}
	}
	if err := validateDefaultsEntry("resource.defaults.value", value.Value, ""); err != nil {
		return err
	}
	if value.Profiles == nil {
		return nil
	}

	orderedKeys := make([]string, 0, len(value.Profiles))
	for key := range value.Profiles {
		orderedKeys = append(orderedKeys, key)
	}
	sort.Strings(orderedKeys)
	for _, key := range orderedKeys {
		if err := ValidateDefaultsProfileName(key); err != nil {
			return err
		}
		if err := validateDefaultsEntry("resource.defaults.profiles."+key, value.Profiles[key], key); err != nil {
			return err
		}
	}
	return nil
}

func ValidateDefaultsMode(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return DefaultsModeInherit, nil
	}
	switch trimmed {
	case DefaultsModeInherit, DefaultsModeIgnore, DefaultsModeReplace:
		return trimmed, nil
	default:
		return "", faults.Invalid(
			fmt.Sprintf("resource.defaults.mode %q is not supported", value),
			nil,
		)
	}
}

func HasDefaultsSpecDirectives(value *DefaultsSpec) bool {
	return value != nil &&
		(strings.TrimSpace(value.Mode) != "" ||
			value.UseProfiles != nil ||
			value.Value != nil ||
			value.Profiles != nil)
}

func CloneDefaultsSpec(value *DefaultsSpec) *DefaultsSpec {
	if value == nil {
		return nil
	}
	cloned := &DefaultsSpec{
		Mode:        value.Mode,
		UseProfiles: cloneStringSlice(value.UseProfiles),
		Value:       cloneDefaultsEntry(value.Value),
		Profiles:    nil,
	}
	if value.Profiles != nil {
		cloned.Profiles = make(map[string]any, len(value.Profiles))
		keys := make([]string, 0, len(value.Profiles))
		for key := range value.Profiles {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			cloned.Profiles[key] = cloneDefaultsEntry(value.Profiles[key])
		}
	}
	return cloned
}

func MergeDefaultsSpec(base *DefaultsSpec, overlay *DefaultsSpec) *DefaultsSpec {
	if base == nil && overlay == nil {
		return nil
	}

	merged := CloneDefaultsSpec(base)
	if merged == nil {
		merged = &DefaultsSpec{}
	}
	if overlay == nil {
		if !HasDefaultsSpecDirectives(merged) {
			return nil
		}
		return merged
	}

	mode, _ := ValidateDefaultsMode(overlay.Mode)
	switch mode {
	case DefaultsModeIgnore:
		merged.Value = nil
		merged.UseProfiles = nil
	case DefaultsModeReplace:
		merged.Value = nil
		merged.UseProfiles = nil
		merged.Profiles = nil
	}

	if strings.TrimSpace(overlay.Mode) != "" {
		merged.Mode = mode
	} else if merged.Mode == "" {
		merged.Mode = DefaultsModeInherit
	}

	if overlay.Profiles != nil {
		if merged.Profiles == nil {
			merged.Profiles = map[string]any{}
		}
		keys := make([]string, 0, len(overlay.Profiles))
		for key := range overlay.Profiles {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			merged.Profiles[key] = mergeDefaultsEntry(merged.Profiles[key], overlay.Profiles[key])
		}
	}
	if overlay.UseProfiles != nil {
		merged.UseProfiles = cloneStringSlice(overlay.UseProfiles)
	}
	if overlay.Value != nil {
		merged.Value = mergeDefaultsEntry(merged.Value, overlay.Value)
	}

	if !HasDefaultsSpecDirectives(merged) {
		return nil
	}
	return merged
}

func ResolveEffectiveDefaults(spec *DefaultsSpec) (resource.Value, error) {
	if spec == nil {
		return map[string]any{}, nil
	}

	current, err := defaultsEntryObject(spec.Value, "resource.defaults.value")
	if err != nil {
		return nil, err
	}
	if current == nil {
		current = map[string]any{}
	}

	for _, profileName := range spec.UseProfiles {
		entry, ok := spec.Profiles[profileName]
		if !ok {
			return nil, faults.Invalid(
				fmt.Sprintf("resource.defaults.useProfiles references unknown profile %q", profileName),
				nil,
			)
		}
		profileValue, err := defaultsEntryObject(entry, "resource.defaults.profiles."+profileName)
		if err != nil {
			return nil, err
		}
		currentValue, mergeErr := resource.MergeWithDefaults(current, profileValue)
		if mergeErr != nil {
			return nil, mergeErr
		}
		current, _ = currentValue.(map[string]any)
		if current == nil {
			current = map[string]any{}
		}
	}

	normalized, err := resource.Normalize(current)
	if err != nil {
		return nil, err
	}
	if normalized == nil {
		return map[string]any{}, nil
	}
	return normalized, nil
}

func validateDefaultsEntry(field string, value any, expectedProfile string) error {
	if value == nil {
		return nil
	}

	if includeFile, ok := value.(string); ok {
		return validateDefaultsIncludeReference(field, includeFile, expectedProfile)
	}

	normalized, err := resource.Normalize(value)
	if err != nil {
		return err
	}
	if _, ok := normalized.(map[string]any); !ok {
		return faults.Invalid(field+" must be an object or exact {{include ...}} reference", nil)
	}
	return nil
}

func validateDefaultsIncludeReference(field string, value string, expectedProfile string) error {
	includeFile, ok := ParseDefaultsIncludeReference(value)
	if !ok {
		return faults.Invalid(field+" must be an exact {{include ...}} reference", nil)
	}
	base := strings.TrimSuffix(includeFile, filepath.Ext(includeFile))
	extension := strings.ToLower(filepath.Ext(includeFile))
	switch extension {
	case ".json", ".properties", ".yaml", ".yml":
	default:
		return faults.Invalid(
			fmt.Sprintf("%s include file %q is not supported", field, includeFile),
			nil,
		)
	}

	expectedBase := "defaults"
	if strings.TrimSpace(expectedProfile) != "" {
		expectedBase = "defaults-" + expectedProfile
	}
	if base != expectedBase {
		return faults.Invalid(
			fmt.Sprintf("%s include file %q is invalid for this defaults entry", field, includeFile),
			nil,
		)
	}
	return nil
}

func defaultsEntryObject(value any, field string) (map[string]any, error) {
	if value == nil {
		return nil, nil
	}
	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}
	objectValue, ok := normalized.(map[string]any)
	if !ok {
		return nil, faults.Invalid(field+" must resolve to an object", nil)
	}
	return objectValue, nil
}

func mergeDefaultsEntry(base any, overlay any) any {
	if overlay == nil {
		return cloneDefaultsEntry(base)
	}

	baseObject, baseErr := defaultsEntryObject(base, "")
	overlayObject, overlayErr := defaultsEntryObject(overlay, "")
	if baseErr == nil && overlayErr == nil {
		merged, err := resource.MergeWithDefaults(baseObject, overlayObject)
		if err == nil {
			return cloneDefaultsEntry(merged)
		}
	}

	return cloneDefaultsEntry(overlay)
}

func cloneDefaultsEntry(value any) any {
	if value == nil {
		return nil
	}
	return resource.DeepCopyValue(value)
}
