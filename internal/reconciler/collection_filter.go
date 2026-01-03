package reconciler

import (
	"strings"

	"declarest/internal/resource"

	"github.com/itchyny/gojq"
)

func applyCollectionFilter(op *resource.OperationMetadata, items []resource.Resource) ([]resource.Resource, error) {
	if op == nil {
		return items, nil
	}
	expr := strings.TrimSpace(op.JQFilter)
	if expr == "" {
		return items, nil
	}

	query, err := gojq.Parse(expr)
	if err != nil {
		return nil, err
	}

	var input []any
	for _, item := range items {
		input = append(input, item.V)
	}

	iter := query.Run(input)
	var results []resource.Resource

	appendValue := func(value any) error {
		if value == nil {
			return nil
		}
		res, err := resource.NewResource(value)
		if err != nil {
			return err
		}
		results = append(results, res)
		return nil
	}

	for {
		value, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := value.(error); ok {
			return nil, err
		}
		if arr, ok := value.([]any); ok {
			for _, entry := range arr {
				if err := appendValue(entry); err != nil {
					return nil, err
				}
			}
			continue
		}
		if err := appendValue(value); err != nil {
			return nil, err
		}
	}

	return results, nil
}
