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

import "strings"

const (
	transformSelect  = "selectAttributes"
	transformExclude = "excludeAttributes"
	transformJQ      = "jqExpression"
)

func CloneTransformSteps(values []TransformStep) []TransformStep {
	if values == nil {
		return nil
	}

	cloned := make([]TransformStep, len(values))
	for idx, value := range values {
		cloned[idx] = TransformStep{
			SelectAttributes:  cloneStringSlice(value.SelectAttributes),
			ExcludeAttributes: cloneStringSlice(value.ExcludeAttributes),
			JQExpression:      value.JQExpression,
		}
	}
	return cloned
}

func transformStepType(step TransformStep) string {
	selected := 0
	kind := ""

	if step.SelectAttributes != nil {
		selected++
		kind = transformSelect
	}
	if step.ExcludeAttributes != nil {
		selected++
		kind = transformExclude
	}
	if strings.TrimSpace(step.JQExpression) != "" {
		selected++
		kind = transformJQ
	}

	if selected != 1 {
		return ""
	}
	return kind
}

func TransformStepType(step TransformStep) string {
	return transformStepType(step)
}

func combineTransformSteps(defaults []TransformStep, operation []TransformStep) []TransformStep {
	if defaults == nil && operation == nil {
		return nil
	}

	combined := make([]TransformStep, 0, len(defaults)+len(operation))
	combined = append(combined, CloneTransformSteps(defaults)...)
	combined = append(combined, CloneTransformSteps(operation)...)
	return combined
}

func OrderedTransformSteps(spec OperationSpec) []TransformStep {
	return CloneTransformSteps(spec.Transforms)
}

func HasTransformJQ(values []TransformStep) bool {
	for _, value := range values {
		if transformStepType(value) == transformJQ {
			return true
		}
	}
	return false
}
