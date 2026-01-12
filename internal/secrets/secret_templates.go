package secrets

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"declarest/internal/resource"
)

const secretTemplatePath = "{{secret .}}"

var secretTemplatePattern = regexp.MustCompile(`^\s*\{\{\s*secret\s+(.+?)\s*\}\}\s*$`)

type secretTemplate struct {
	Key     string
	UsePath bool
}

func ParseSecretTemplate(raw string) (secretTemplate, bool) {
	matches := secretTemplatePattern.FindStringSubmatch(raw)
	if len(matches) != 2 {
		return secretTemplate{}, false
	}
	arg := strings.TrimSpace(matches[1])
	if arg == "." {
		return secretTemplate{UsePath: true}, true
	}
	if len(arg) >= 2 {
		if (arg[0] == '\'' && arg[len(arg)-1] == '\'') || (arg[0] == '"' && arg[len(arg)-1] == '"') {
			return secretTemplate{Key: arg[1 : len(arg)-1]}, true
		}
	}
	return secretTemplate{}, false
}

func MaskResourceSecrets(res resource.Resource, resourcePath string, secretPaths []string, manager SecretsManager, store bool) (resource.Resource, error) {
	paths := normalizeSecretPaths(secretPaths)
	if len(paths) == 0 {
		return res, nil
	}
	if store && manager == nil {
		return resource.Resource{}, ErrSecretStoreNotConfigured
	}

	if res.Kind() == resource.KindArray {
		if store {
			return resource.Resource{}, errors.New("cannot store secrets for collection resources; save collection items instead")
		}
		items, ok := res.AsArray()
		if !ok {
			return res, nil
		}
		masked := make([]any, len(items))
		for i, item := range items {
			sub, err := resource.NewResource(item)
			if err != nil {
				return resource.Resource{}, err
			}
			updated, err := maskObjectSecrets(sub, resourcePath, paths, manager, store)
			if err != nil {
				return resource.Resource{}, err
			}
			masked[i] = updated.V
		}
		return resource.NewResource(masked)
	}

	return maskObjectSecrets(res, resourcePath, paths, manager, store)
}

func SecretPlaceholders(res resource.Resource, secretPaths []string) (map[string]string, error) {
	paths := normalizeSecretPaths(secretPaths)
	if len(paths) == 0 {
		return nil, nil
	}

	placeholders := make(map[string]string)

	if res.Kind() == resource.KindArray {
		items, ok := res.AsArray()
		if !ok {
			return placeholders, nil
		}
		for _, item := range items {
			sub, err := resource.NewResource(item)
			if err != nil {
				return nil, err
			}
			found, err := SecretPlaceholders(sub, paths)
			if err != nil {
				return nil, err
			}
			for key, value := range found {
				if _, ok := placeholders[key]; !ok {
					placeholders[key] = value
				}
			}
		}
		return placeholders, nil
	}

	obj, ok := res.AsObject()
	if !ok {
		return placeholders, nil
	}

	for _, path := range paths {
		tokens, err := parseSecretPath(path)
		if err != nil {
			return nil, err
		}
		value, ok := getValueAtPath(obj, tokens)
		if !ok {
			continue
		}
		raw, ok := value.(string)
		if !ok {
			continue
		}
		if _, ok := ParseSecretTemplate(raw); ok {
			placeholders[path] = raw
		}
	}

	return placeholders, nil
}

func NormalizeResourceSecrets(res resource.Resource, secretPaths []string, placeholders map[string]string) (resource.Resource, error) {
	paths := normalizeSecretPaths(secretPaths)
	if len(paths) == 0 {
		return res, nil
	}

	if res.Kind() == resource.KindArray {
		items, ok := res.AsArray()
		if !ok {
			return res, nil
		}
		masked := make([]any, len(items))
		for i, item := range items {
			sub, err := resource.NewResource(item)
			if err != nil {
				return resource.Resource{}, err
			}
			updated, err := NormalizeResourceSecrets(sub, paths, placeholders)
			if err != nil {
				return resource.Resource{}, err
			}
			masked[i] = updated.V
		}
		return resource.NewResource(masked)
	}

	updated := res.Clone()
	obj, ok := updated.AsObject()
	if !ok {
		return res, nil
	}

	for _, path := range paths {
		tokens, err := parseSecretPath(path)
		if err != nil {
			return resource.Resource{}, err
		}
		if _, ok := getValueAtPath(obj, tokens); !ok {
			continue
		}
		placeholder := secretTemplatePath
		if placeholders != nil {
			if raw := strings.TrimSpace(placeholders[path]); raw != "" {
				placeholder = raw
			}
		}
		if !setValueAtPath(obj, tokens, placeholder) {
			return resource.Resource{}, fmt.Errorf("failed to set secret placeholder for %q", path)
		}
	}

	return updated, nil
}

func ResolveResourceSecrets(res resource.Resource, resourcePath string, secretPaths []string, manager SecretsManager) (resource.Resource, error) {
	paths := normalizeSecretPaths(secretPaths)
	if len(paths) == 0 {
		return res, nil
	}
	if manager == nil {
		return resource.Resource{}, ErrSecretStoreNotConfigured
	}

	if res.Kind() == resource.KindArray {
		return resource.Resource{}, errors.New("cannot resolve secrets for collection resources without item paths")
	}

	updated := res.Clone()
	obj, ok := updated.AsObject()
	if !ok {
		return res, nil
	}

	normalizedPath := resource.NormalizePath(resourcePath)
	for _, path := range paths {
		tokens, err := parseSecretPath(path)
		if err != nil {
			return resource.Resource{}, err
		}
		value, ok := getValueAtPath(obj, tokens)
		if !ok {
			continue
		}
		raw, ok := value.(string)
		if !ok {
			continue
		}
		template, ok := ParseSecretTemplate(raw)
		if !ok {
			continue
		}
		key := template.Key
		if template.UsePath {
			key = path
		}
		secret, err := manager.GetSecret(normalizedPath, key)
		if err != nil {
			return resource.Resource{}, err
		}
		if !setValueAtPath(obj, tokens, secret) {
			return resource.Resource{}, fmt.Errorf("failed to set secret for %q", path)
		}
	}

	return updated, nil
}

func HasSecretPlaceholders(res resource.Resource, secretPaths []string) (bool, error) {
	paths := normalizeSecretPaths(secretPaths)
	if len(paths) == 0 {
		return false, nil
	}

	if res.Kind() == resource.KindArray {
		items, ok := res.AsArray()
		if !ok {
			return false, nil
		}
		for _, item := range items {
			sub, err := resource.NewResource(item)
			if err != nil {
				return false, err
			}
			has, err := HasSecretPlaceholders(sub, paths)
			if err != nil || has {
				return has, err
			}
		}
		return false, nil
	}

	obj, ok := res.AsObject()
	if !ok {
		return false, nil
	}

	for _, path := range paths {
		tokens, err := parseSecretPath(path)
		if err != nil {
			return false, err
		}
		value, ok := getValueAtPath(obj, tokens)
		if !ok {
			continue
		}
		raw, ok := value.(string)
		if !ok {
			continue
		}
		if _, ok := ParseSecretTemplate(raw); ok {
			return true, nil
		}
	}
	return false, nil
}

func maskObjectSecrets(res resource.Resource, resourcePath string, secretPaths []string, manager SecretsManager, store bool) (resource.Resource, error) {
	updated := res.Clone()
	obj, ok := updated.AsObject()
	if !ok {
		return res, nil
	}

	normalizedPath := resource.NormalizePath(resourcePath)
	for _, path := range secretPaths {
		tokens, err := parseSecretPath(path)
		if err != nil {
			return resource.Resource{}, err
		}
		value, ok := getValueAtPath(obj, tokens)
		if !ok {
			continue
		}
		raw, ok := value.(string)
		if ok {
			if _, isTemplate := ParseSecretTemplate(raw); isTemplate {
				continue
			}
		}
		if store {
			secretValue, err := secretValueString(value)
			if err != nil {
				return resource.Resource{}, fmt.Errorf("secret %q: %w", path, err)
			}
			if err := manager.UpdateSecret(normalizedPath, path, secretValue); err != nil {
				return resource.Resource{}, err
			}
		}
		if !setValueAtPath(obj, tokens, secretTemplatePath) {
			return resource.Resource{}, fmt.Errorf("failed to set secret placeholder for %q", path)
		}
	}

	return updated, nil
}

func secretValueString(value any) (string, error) {
	switch v := value.(type) {
	case string:
		return v, nil
	case json.Number:
		return v.String(), nil
	case fmt.Stringer:
		return v.String(), nil
	default:
		return "", errors.New("value must be a string")
	}
}

func normalizeSecretPaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	var out []string
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

type secretPathToken struct {
	key     string
	index   int
	isIndex bool
}

func parseSecretPath(raw string) ([]secretPathToken, error) {
	path := strings.TrimSpace(raw)
	if path == "" {
		return nil, errors.New("secret path is empty")
	}

	var tokens []secretPathToken
	for i := 0; i < len(path); {
		switch path[i] {
		case '.':
			i++
			continue
		case '[':
			i++
			start := i
			for i < len(path) && path[i] != ']' {
				if path[i] < '0' || path[i] > '9' {
					return nil, fmt.Errorf("invalid index in %q", raw)
				}
				i++
			}
			if i >= len(path) || path[i] != ']' || start == i {
				return nil, fmt.Errorf("invalid index in %q", raw)
			}
			idx, err := strconv.Atoi(path[start:i])
			if err != nil {
				return nil, fmt.Errorf("invalid index in %q", raw)
			}
			tokens = append(tokens, secretPathToken{isIndex: true, index: idx})
			i++
			continue
		default:
			start := i
			for i < len(path) && path[i] != '.' && path[i] != '[' {
				i++
			}
			if start == i {
				return nil, fmt.Errorf("invalid secret path %q", raw)
			}
			tokens = append(tokens, secretPathToken{key: path[start:i]})
		}
	}

	return tokens, nil
}

func getValueAtPath(root map[string]any, tokens []secretPathToken) (any, bool) {
	var current any = root
	for _, token := range tokens {
		if token.isIndex {
			arr, ok := current.([]any)
			if !ok || token.index < 0 || token.index >= len(arr) {
				return nil, false
			}
			current = arr[token.index]
			continue
		}
		obj, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := obj[token.key]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func setValueAtPath(root map[string]any, tokens []secretPathToken, value any) bool {
	var current any = root
	for idx, token := range tokens {
		last := idx == len(tokens)-1
		if token.isIndex {
			arr, ok := current.([]any)
			if !ok || token.index < 0 || token.index >= len(arr) {
				return false
			}
			if last {
				arr[token.index] = value
				return true
			}
			current = arr[token.index]
			continue
		}
		obj, ok := current.(map[string]any)
		if !ok {
			return false
		}
		if last {
			if _, exists := obj[token.key]; !exists {
				return false
			}
			obj[token.key] = value
			return true
		}
		next, ok := obj[token.key]
		if !ok {
			return false
		}
		current = next
	}
	return false
}
