package metadata

import "strings"

const (
	payloadTransformStepFilter   = "filterAttributes"
	payloadTransformStepSuppress = "suppressAttributes"
	payloadTransformStepJQ       = "jqExpression"
)

func payloadTransformStepForKey(key string) (string, bool) {
	switch key {
	case "filterAttributes", "filter":
		return payloadTransformStepFilter, true
	case "suppressAttributes", "suppress", "ignoreAttributes":
		return payloadTransformStepSuppress, true
	case "jqExpression", "jq":
		return payloadTransformStepJQ, true
	default:
		return "", false
	}
}

func normalizePayloadTransformOrder(order []string) []string {
	if len(order) == 0 {
		return nil
	}

	out := make([]string, 0, len(order))
	seen := map[string]struct{}{}
	for _, step := range order {
		switch step {
		case payloadTransformStepFilter, payloadTransformStepSuppress, payloadTransformStepJQ:
			if _, exists := seen[step]; exists {
				continue
			}
			seen[step] = struct{}{}
			out = append(out, step)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func mergePayloadTransformOrder(base []string, overlay []string) []string {
	baseOrder := normalizePayloadTransformOrder(base)
	overlayOrder := normalizePayloadTransformOrder(overlay)
	if len(baseOrder) == 0 && len(overlayOrder) == 0 {
		return nil
	}
	if len(overlayOrder) == 0 {
		return cloneStringSlice(baseOrder)
	}
	if len(baseOrder) == 0 {
		return cloneStringSlice(overlayOrder)
	}

	remove := make(map[string]struct{}, len(overlayOrder))
	for _, step := range overlayOrder {
		remove[step] = struct{}{}
	}

	merged := make([]string, 0, len(baseOrder)+len(overlayOrder))
	for _, step := range baseOrder {
		if _, exists := remove[step]; exists {
			continue
		}
		merged = append(merged, step)
	}
	merged = append(merged, overlayOrder...)
	return merged
}

func OrderedPayloadTransformSteps(spec OperationSpec) []string {
	steps := normalizePayloadTransformOrder(spec.PayloadTransformOrder)

	hasFilter := spec.Filter != nil
	hasSuppress := spec.Suppress != nil
	hasJQ := strings.TrimSpace(spec.JQ) != ""

	if !hasFilter && !hasSuppress && !hasJQ {
		return nil
	}

	seen := make(map[string]struct{}, len(steps))
	ordered := make([]string, 0, 3)

	addIfNeeded := func(step string, enabled bool) {
		if !enabled {
			return
		}
		if _, exists := seen[step]; exists {
			return
		}
		seen[step] = struct{}{}
		ordered = append(ordered, step)
	}

	for _, step := range steps {
		switch step {
		case payloadTransformStepFilter:
			addIfNeeded(step, hasFilter)
		case payloadTransformStepSuppress:
			addIfNeeded(step, hasSuppress)
		case payloadTransformStepJQ:
			addIfNeeded(step, hasJQ)
		}
	}

	addIfNeeded(payloadTransformStepFilter, hasFilter)
	addIfNeeded(payloadTransformStepSuppress, hasSuppress)
	addIfNeeded(payloadTransformStepJQ, hasJQ)

	return ordered
}
