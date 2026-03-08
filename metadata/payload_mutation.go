package metadata

import "strings"

const (
	payloadMutationSelect   = "selectAttributes"
	payloadMutationSuppress = "suppressAttributes"
	payloadMutationJQ       = "jqExpression"
)

func clonePayloadMutationSteps(values []PayloadMutationStep) []PayloadMutationStep {
	if values == nil {
		return nil
	}

	cloned := make([]PayloadMutationStep, len(values))
	for idx, value := range values {
		cloned[idx] = PayloadMutationStep{
			SelectAttributes:   cloneStringSlice(value.SelectAttributes),
			SuppressAttributes: cloneStringSlice(value.SuppressAttributes),
			JQExpression:       value.JQExpression,
		}
	}
	return cloned
}

func payloadMutationStepType(step PayloadMutationStep) string {
	selected := 0
	kind := ""

	if step.SelectAttributes != nil {
		selected++
		kind = payloadMutationSelect
	}
	if step.SuppressAttributes != nil {
		selected++
		kind = payloadMutationSuppress
	}
	if strings.TrimSpace(step.JQExpression) != "" {
		selected++
		kind = payloadMutationJQ
	}

	if selected != 1 {
		return ""
	}
	return kind
}

func PayloadMutationStepType(step PayloadMutationStep) string {
	return payloadMutationStepType(step)
}

func hasPayloadMutationSteps(values []PayloadMutationStep) bool {
	return values != nil
}

func combinePayloadMutationSteps(defaults []PayloadMutationStep, operation []PayloadMutationStep) []PayloadMutationStep {
	if defaults == nil && operation == nil {
		return nil
	}

	combined := make([]PayloadMutationStep, 0, len(defaults)+len(operation))
	combined = append(combined, clonePayloadMutationSteps(defaults)...)
	combined = append(combined, clonePayloadMutationSteps(operation)...)
	return combined
}

func OrderedPayloadMutationSteps(spec OperationSpec) []PayloadMutationStep {
	return clonePayloadMutationSteps(spec.PayloadMutation)
}

func HasPayloadMutationJQ(values []PayloadMutationStep) bool {
	for _, value := range values {
		if payloadMutationStepType(value) == payloadMutationJQ {
			return true
		}
	}
	return false
}
