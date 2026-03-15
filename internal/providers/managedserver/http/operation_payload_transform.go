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

package http

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	metadatavalidation "github.com/crmarques/declarest/metadata/validation"
	"github.com/crmarques/declarest/resource"
)

func (g *Client) applyOperationPayloadTransforms(
	ctx context.Context,
	payload any,
	spec metadata.OperationSpec,
) (resource.Value, error) {
	payload = unwrapContentValue(payload)
	steps := metadata.OrderedTransformSteps(spec)
	if len(steps) == 0 {
		return payload, nil
	}

	normalized, err := resource.Normalize(payload)
	if err != nil {
		return nil, err
	}

	current := normalized
	for _, step := range steps {
		switch metadata.TransformStepType(step) {
		case "selectAttributes":
			current, err = applyPayloadSelectAttributes(current, step.SelectAttributes)
		case "excludeAttributes":
			current, err = applyPayloadExcludeAttributes(current, step.ExcludeAttributes)
		case "jqExpression":
			current, err = g.applyPayloadJQ(ctx, current, step.JQExpression)
		}
		if err != nil {
			return nil, err
		}
	}

	return resource.Normalize(current)
}

func applyPayloadSelectAttributes(value resource.Value, attributes []string) (resource.Value, error) {
	pointers, err := metadatavalidation.NormalizeAttributePointers("selectAttributes", attributes)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}

	filtered := any(nil)
	for _, pointer := range pointers {
		if pointer == "" {
			return resource.DeepCopyValue(value), nil
		}

		item, found, err := resource.LookupJSONPointer(value, pointer)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		filtered, err = resource.SetJSONPointerValue(filtered, pointer, item)
		if err != nil {
			return nil, err
		}
	}

	if filtered == nil {
		switch value.(type) {
		case []any:
			return []any{}, nil
		case map[string]any:
			return map[string]any{}, nil
		default:
			return nil, nil
		}
	}

	return filtered, nil
}

func applyPayloadExcludeAttributes(value resource.Value, attributes []string) (resource.Value, error) {
	pointers, err := metadatavalidation.NormalizeAttributePointers("excludeAttributes", attributes)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}

	filtered := resource.DeepCopyValue(value)
	for _, pointer := range pointers {
		if pointer == "" {
			return nil, nil
		}

		filtered, err = resource.DeleteJSONPointerValue(filtered, pointer)
		if err != nil {
			return nil, err
		}
	}

	return filtered, nil
}
func (g *Client) applyPayloadJQ(ctx context.Context, payload resource.Value, expression string) (resource.Value, error) {
	trimmedExpression := strings.TrimSpace(expression)
	if trimmedExpression == "" {
		return payload, nil
	}

	code, err := g.compileListJQCode(ctx, trimmedExpression)
	if err != nil {
		return nil, faults.NewValidationError("invalid payload jq expression", err)
	}

	runCtx := ctx
	if runCtx == nil {
		runCtx = context.Background()
	}

	iterator := code.RunWithContext(runCtx, payload)
	results := make([]any, 0, 1)
	for {
		value, ok := iterator.Next()
		if !ok {
			break
		}
		if valueErr, isErr := value.(error); isErr {
			return nil, faults.NewValidationError("failed to evaluate payload jq expression", valueErr)
		}
		results = append(results, value)
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
