package http

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (g *HTTPResourceServerGateway) applyOperationPayloadTransforms(
	ctx context.Context,
	payload any,
	spec metadata.OperationSpec,
) (resource.Value, error) {
	steps := metadata.OrderedPayloadTransformSteps(spec)
	if len(steps) == 0 {
		return payload, nil
	}

	normalized, err := resource.Normalize(payload)
	if err != nil {
		return nil, err
	}

	current := normalized
	for _, step := range steps {
		switch step {
		case "filterAttributes":
			current, err = applyPayloadFilterAttributes(current, spec.Filter)
		case "suppressAttributes":
			current, err = applyPayloadSuppressAttributes(current, spec.Suppress)
		case "jqExpression":
			current, err = g.applyPayloadJQ(ctx, current, spec.JQ)
		default:
			continue
		}
		if err != nil {
			return nil, err
		}
	}

	return resource.Normalize(current)
}

func applyPayloadFilterAttributes(value resource.Value, attributes []string) (resource.Value, error) {
	if value == nil {
		return nil, nil
	}

	objectValue, ok := value.(map[string]any)
	if !ok {
		return nil, validationError("payload filterAttributes requires an object payload", nil)
	}

	names, err := normalizePayloadAttributeNames("filterAttributes", attributes)
	if err != nil {
		return nil, err
	}

	filtered := make(map[string]any, len(names))
	for _, name := range names {
		item, found := objectValue[name]
		if !found {
			continue
		}
		filtered[name] = item
	}

	return filtered, nil
}

func applyPayloadSuppressAttributes(value resource.Value, attributes []string) (resource.Value, error) {
	if value == nil {
		return nil, nil
	}

	objectValue, ok := value.(map[string]any)
	if !ok {
		return nil, validationError("payload suppressAttributes requires an object payload", nil)
	}

	names, err := normalizePayloadAttributeNames("suppressAttributes", attributes)
	if err != nil {
		return nil, err
	}

	filtered := make(map[string]any, len(objectValue))
	for key, item := range objectValue {
		filtered[key] = item
	}
	for _, name := range names {
		delete(filtered, name)
	}

	return filtered, nil
}

func normalizePayloadAttributeNames(field string, attributes []string) ([]string, error) {
	if attributes == nil {
		return nil, nil
	}

	names := make([]string, 0, len(attributes))
	seen := make(map[string]struct{}, len(attributes))
	for _, raw := range attributes {
		name := strings.TrimSpace(raw)
		if name == "" {
			return nil, validationError("payload "+field+" contains an empty attribute name", nil)
		}
		if _, exists := seen[name]; exists {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}

	return names, nil
}

func (g *HTTPResourceServerGateway) applyPayloadJQ(ctx context.Context, payload resource.Value, expression string) (resource.Value, error) {
	trimmedExpression := strings.TrimSpace(expression)
	if trimmedExpression == "" {
		return payload, nil
	}

	code, err := g.compileListJQCode(ctx, trimmedExpression)
	if err != nil {
		return nil, validationError("invalid payload jq expression", err)
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
			return nil, validationError("failed to evaluate payload jq expression", valueErr)
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
