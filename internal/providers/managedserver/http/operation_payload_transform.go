package http

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
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
	pointers, err := normalizePayloadAttributePointers("selectAttributes", attributes)
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
	pointers, err := normalizePayloadAttributePointers("excludeAttributes", attributes)
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

func normalizePayloadAttributePointers(field string, attributes []string) ([]string, error) {
	if attributes == nil {
		return nil, nil
	}

	pointers := make([]string, 0, len(attributes))
	seen := make(map[string]struct{}, len(attributes))
	for _, raw := range attributes {
		pointer := strings.TrimSpace(raw)
		if pointer == "" {
			return nil, faults.NewValidationError("payload "+field+" contains an empty JSON pointer", nil)
		}
		if _, err := resource.ParseJSONPointer(pointer); err != nil {
			return nil, faults.NewValidationError("payload "+field+" contains an invalid JSON pointer", err)
		}
		if _, exists := seen[pointer]; exists {
			continue
		}
		seen[pointer] = struct{}{}
		pointers = append(pointers, pointer)
	}

	return pointers, nil
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
