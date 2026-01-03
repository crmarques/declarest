package resource

import "strings"

// ApplyCompareRules prepares a resource for comparison by applying ignore/filter/suppress/jq rules.
func ApplyCompareRules(res Resource, cmp *CompareMetadata) (Resource, error) {
	if cmp == nil {
		return res, nil
	}

	current := res.Clone()

	if obj, ok := current.AsObject(); ok {
		if cmp.IgnoreAttributes != nil {
			for _, attr := range cmp.IgnoreAttributes {
				key := strings.TrimSpace(attr)
				if key == "" {
					continue
				}
				delete(obj, key)
			}
			current.V = obj
		}
	}

	if len(cmp.FilterAttributes) > 0 {
		if obj, ok := current.AsObject(); ok {
			current.V = filterAttributes(obj, cmp.FilterAttributes)
		}
	}

	if cmp.SuppressAttributes != nil {
		if obj, ok := current.AsObject(); ok {
			current.V = suppressAttributes(obj, cmp.SuppressAttributes)
		}
	}

	if expr := strings.TrimSpace(cmp.JQExpression); expr != "" {
		value, err := executeJQ(current.V, expr)
		if err != nil {
			return Resource{}, err
		}
		normalized, err := NewResource(value)
		if err != nil {
			return Resource{}, err
		}
		return normalized, nil
	}

	return current, nil
}
