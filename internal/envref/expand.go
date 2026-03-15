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

package envref

import (
	"os"
	"reflect"
	"regexp"
	"strings"
)

var exactPlaceholderPattern = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)

func ExpandExactEnvPlaceholders[T any](value T) T {
	expanded := cloneValue(reflect.ValueOf(value))
	if !expanded.IsValid() {
		return value
	}
	expandValue(expanded)
	return expanded.Interface().(T)
}

func ExpandExactEnvPlaceholdersInPlace(target any) {
	if target == nil {
		return
	}

	value := reflect.ValueOf(target)
	if value.Kind() != reflect.Pointer || value.IsNil() {
		return
	}

	expandValue(value.Elem())
}

func expandValue(value reflect.Value) {
	if !value.IsValid() {
		return
	}

	switch value.Kind() {
	case reflect.Pointer:
		if !value.IsNil() {
			expandValue(value.Elem())
		}
	case reflect.Interface:
		if value.IsNil() {
			return
		}
		expanded := reflect.New(value.Elem().Type()).Elem()
		expanded.Set(value.Elem())
		expandValue(expanded)
		if value.CanSet() {
			value.Set(expanded)
		}
	case reflect.Struct:
		for idx := 0; idx < value.NumField(); idx++ {
			field := value.Field(idx)
			if !field.CanSet() && field.Kind() != reflect.Struct && field.Kind() != reflect.Pointer && field.Kind() != reflect.Interface {
				continue
			}
			expandValue(field)
		}
	case reflect.Slice, reflect.Array:
		for idx := 0; idx < value.Len(); idx++ {
			expandValue(value.Index(idx))
		}
	case reflect.Map:
		if value.IsNil() {
			return
		}
		for _, key := range value.MapKeys() {
			current := value.MapIndex(key)
			expanded := reflect.New(current.Type()).Elem()
			expanded.Set(current)
			expandValue(expanded)
			value.SetMapIndex(key, expanded)
		}
	case reflect.String:
		if !value.CanSet() {
			return
		}
		if resolved, ok := resolvePlaceholder(value.String()); ok {
			value.SetString(resolved)
		}
	}
}

func resolvePlaceholder(value string) (string, bool) {
	matches := exactPlaceholderPattern.FindStringSubmatch(strings.TrimSpace(value))
	if len(matches) != 2 {
		return "", false
	}
	return os.Getenv(matches[1]), true
}

func cloneValue(value reflect.Value) reflect.Value {
	if !value.IsValid() {
		return reflect.Value{}
	}

	switch value.Kind() {
	case reflect.Pointer:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.New(value.Type().Elem())
		cloned.Elem().Set(cloneValue(value.Elem()))
		return cloned
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := cloneValue(value.Elem())
		wrapped := reflect.New(value.Type()).Elem()
		wrapped.Set(cloned)
		return wrapped
	case reflect.Struct:
		cloned := reflect.New(value.Type()).Elem()
		cloned.Set(value)
		for idx := 0; idx < value.NumField(); idx++ {
			field := value.Field(idx)
			clonedField := cloned.Field(idx)
			if !clonedField.CanSet() {
				continue
			}
			switch field.Kind() {
			case reflect.Pointer, reflect.Interface, reflect.Struct, reflect.Slice, reflect.Array, reflect.Map:
				clonedField.Set(cloneValue(field))
			}
		}
		return cloned
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for idx := 0; idx < value.Len(); idx++ {
			cloned.Index(idx).Set(cloneValue(value.Index(idx)))
		}
		return cloned
	case reflect.Array:
		cloned := reflect.New(value.Type()).Elem()
		for idx := 0; idx < value.Len(); idx++ {
			cloned.Index(idx).Set(cloneValue(value.Index(idx)))
		}
		return cloned
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		cloned := reflect.MakeMapWithSize(value.Type(), value.Len())
		for _, key := range value.MapKeys() {
			cloned.SetMapIndex(key, cloneValue(value.MapIndex(key)))
		}
		return cloned
	default:
		return value
	}
}
