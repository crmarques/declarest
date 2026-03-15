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

package cliutil

import (
	"strings"

	"github.com/crmarques/declarest/resource"
)

func ParsePointerAssignmentsObject(raw string) (map[string]any, error) {
	output := map[string]any{}
	if err := ApplyPointerAssignmentsObject(output, raw); err != nil {
		return nil, err
	}
	return output, nil
}

func ApplyPointerAssignmentsObject(target map[string]any, raw string) error {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ValidationError("invalid assignment list: expected <json-pointer>=<value>", nil)
	}

	for _, item := range strings.Split(trimmed, ",") {
		part := strings.TrimSpace(item)
		if part == "" {
			return ValidationError("invalid assignment list: empty item", nil)
		}
		pieces := strings.SplitN(part, "=", 2)
		if len(pieces) != 2 {
			return ValidationError("invalid assignment list: expected <json-pointer>=<value>", nil)
		}

		pointer := strings.TrimSpace(pieces[0])
		if pointer == "" {
			return ValidationError("invalid assignment list: JSON pointer must not be empty", nil)
		}
		if _, err := resource.ParseJSONPointer(pointer); err != nil {
			return ValidationError("invalid assignment list: JSON pointer is invalid", err)
		}

		value := strings.TrimSpace(pieces[1])
		if _, err := resource.SetJSONPointerValue(target, pointer, value); err != nil {
			return ValidationError("invalid assignment list: JSON pointer conflicts with scalar value", err)
		}
	}

	return nil
}
