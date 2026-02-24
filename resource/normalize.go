package resource

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"sort"

	"github.com/crmarques/declarest/faults"
)

func Normalize(value Value) (Value, error) {
	normalized, err := normalizeValue(value)
	if err != nil {
		return nil, err
	}
	return normalized, nil
}

func normalizeValue(value any) (any, error) {
	switch typed := value.(type) {
	case nil, bool, string:
		return typed, nil
	case float32:
		return normalizeFloat(float64(typed))
	case float64:
		return normalizeFloat(typed)
	case int:
		return int64(typed), nil
	case int8:
		return int64(typed), nil
	case int16:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case uint:
		return normalizeUint(uint64(typed))
	case uint8:
		return normalizeUint(uint64(typed))
	case uint16:
		return normalizeUint(uint64(typed))
	case uint32:
		return normalizeUint(uint64(typed))
	case uint64:
		return normalizeUint(typed)
	case json.Number:
		return normalizeJSONNumber(typed)
	case []any:
		return normalizeSlice(typed)
	case map[string]any:
		return normalizeStringMap(typed)
	}

	return normalizeReflectValue(value)
}

func normalizeFloat(value float64) (float64, error) {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, faults.NewTypedError(faults.ValidationError, "payload contains non-finite float", nil)
	}
	return value, nil
}

func normalizeUint(value uint64) (int64, error) {
	if value > math.MaxInt64 {
		return 0, faults.NewTypedError(faults.ValidationError, "payload contains integer out of range", nil)
	}
	return int64(value), nil
}

func normalizeJSONNumber(value json.Number) (any, error) {
	if asInt, err := value.Int64(); err == nil {
		return asInt, nil
	}
	asBig, ok := new(big.Int).SetString(value.String(), 10)
	if ok {
		if asBig.IsInt64() {
			return asBig.Int64(), nil
		}
		return nil, faults.NewTypedError(faults.ValidationError, "payload contains integer out of range", nil)
	}

	asFloat, err := value.Float64()
	if err != nil {
		return nil, faults.NewTypedError(faults.ValidationError, "payload contains invalid number", err)
	}
	return normalizeFloat(asFloat)
}

func normalizeSlice(values []any) ([]any, error) {
	normalized := make([]any, len(values))
	for idx, item := range values {
		itemValue, err := normalizeValue(item)
		if err != nil {
			return nil, err
		}
		normalized[idx] = itemValue
	}
	return normalized, nil
}

func normalizeStringMap(values map[string]any) (map[string]any, error) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	normalized := make(map[string]any, len(values))
	for _, key := range keys {
		itemValue, err := normalizeValue(values[key])
		if err != nil {
			return nil, err
		}
		normalized[key] = itemValue
	}

	return normalized, nil
}

func normalizeReflectValue(value any) (any, error) {
	if value == nil {
		return nil, nil
	}

	reflectValue := reflect.ValueOf(value)
	switch reflectValue.Kind() {
	case reflect.Map:
		if reflectValue.Type().Key().Kind() != reflect.String {
			return nil, faults.NewTypedError(faults.ValidationError, "payload map keys must be strings", nil)
		}

		keys := reflectValue.MapKeys()
		stringKeys := make([]string, len(keys))
		for idx, key := range keys {
			stringKeys[idx] = key.String()
		}
		sort.Strings(stringKeys)

		normalized := make(map[string]any, len(stringKeys))
		for _, key := range stringKeys {
			mapKey := reflect.ValueOf(key)
			if mapKey.Type() != reflectValue.Type().Key() {
				mapKey = mapKey.Convert(reflectValue.Type().Key())
			}
			itemValue := reflectValue.MapIndex(mapKey)
			result, err := normalizeValue(itemValue.Interface())
			if err != nil {
				return nil, err
			}
			normalized[key] = result
		}
		return normalized, nil
	case reflect.Slice, reflect.Array:
		length := reflectValue.Len()
		normalized := make([]any, length)
		for idx := range length {
			result, err := normalizeValue(reflectValue.Index(idx).Interface())
			if err != nil {
				return nil, err
			}
			normalized[idx] = result
		}
		return normalized, nil
	default:
		return nil, faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("unsupported payload type %T", value),
			nil,
		)
	}
}
