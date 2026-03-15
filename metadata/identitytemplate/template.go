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

package identitytemplate

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/crmarques/declarest/resource"
)

type Template struct {
	raw           string
	parts         []templatePart
	pointers      []string
	simplePointer string
}

type templatePart interface {
	render(payload any) (string, error)
	collectPointers(add func(string))
}

type literalPart struct {
	text string
}

type expressionPart struct {
	raw  string
	node expressionNode
}

type expressionNode interface {
	eval(payload any) (string, bool, error)
	collectPointers(add func(string))
}

type literalNode struct {
	value string
}

type pointerNode struct {
	pointer string
}

type helperNode struct {
	name string
	args []expressionNode
}

type token struct {
	text   string
	quoted bool
}

type cacheEntry struct {
	template *Template
	err      error
}

var compiledTemplateCache sync.Map
var barePointerIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

func Compile(raw string) (*Template, error) {
	if cached, ok := compiledTemplateCache.Load(raw); ok {
		entry := cached.(cacheEntry)
		return entry.template, entry.err
	}

	compiled, err := compile(raw)
	compiledTemplateCache.Store(raw, cacheEntry{template: compiled, err: err})
	return compiled, err
}

func Render(raw string, payload any) (string, error) {
	compiled, err := Compile(raw)
	if err != nil {
		return "", err
	}
	return compiled.Render(payload)
}

func ExtractPointers(raw string) ([]string, error) {
	compiled, err := Compile(raw)
	if err != nil {
		return nil, err
	}
	return compiled.Pointers(), nil
}

func SimplePointer(raw string) (string, bool, error) {
	compiled, err := Compile(raw)
	if err != nil {
		return "", false, err
	}
	pointer, ok := compiled.SimplePointer()
	return pointer, ok, nil
}

func PointerTemplate(pointer string) string {
	return "{{" + strings.TrimSpace(pointer) + "}}"
}

func (t *Template) Raw() string {
	if t == nil {
		return ""
	}
	return t.raw
}

func (t *Template) Render(payload any) (string, error) {
	if t == nil {
		return "", nil
	}

	var builder strings.Builder
	for _, part := range t.parts {
		rendered, err := part.render(payload)
		if err != nil {
			return "", err
		}
		builder.WriteString(rendered)
	}

	return builder.String(), nil
}

func (t *Template) Pointers() []string {
	if t == nil || len(t.pointers) == 0 {
		return nil
	}
	return append([]string(nil), t.pointers...)
}

func (t *Template) SimplePointer() (string, bool) {
	if t == nil || strings.TrimSpace(t.simplePointer) == "" {
		return "", false
	}
	return t.simplePointer, true
}

func compile(raw string) (*Template, error) {
	source := normalizePointerShorthand(raw)
	compiled := &Template{
		raw:   raw,
		parts: make([]templatePart, 0, 4),
	}

	addPointer := orderedPointerCollector(&compiled.pointers)
	offset := 0
	for {
		start := strings.Index(source[offset:], "{{")
		if start < 0 {
			if offset < len(source) {
				compiled.parts = append(compiled.parts, literalPart{text: source[offset:]})
			}
			break
		}
		start += offset

		if start > offset {
			compiled.parts = append(compiled.parts, literalPart{text: source[offset:start]})
		}

		end := strings.Index(source[start+2:], "}}")
		if end < 0 {
			return nil, fmt.Errorf("identity template contains an unterminated expression")
		}
		end += start + 2

		rawExpression := strings.TrimSpace(source[start+2 : end])
		if rawExpression == "" {
			return nil, fmt.Errorf("identity template contains an empty expression")
		}

		node, err := parseExpression(rawExpression)
		if err != nil {
			return nil, err
		}
		part := expressionPart{raw: rawExpression, node: node}
		part.collectPointers(addPointer)
		compiled.parts = append(compiled.parts, part)

		offset = end + 2
	}

	if len(compiled.parts) == 1 {
		if part, ok := compiled.parts[0].(expressionPart); ok {
			if pointer, ok := part.node.(pointerNode); ok {
				compiled.simplePointer = pointer.pointer
			}
		}
	}

	return compiled, nil
}

func normalizePointerShorthand(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || strings.Contains(trimmed, "{{") {
		return raw
	}
	if _, err := resource.ParseJSONPointer(trimmed); err == nil {
		return PointerTemplate(trimmed)
	}
	return raw
}

func orderedPointerCollector(target *[]string) func(string) {
	seen := make(map[string]struct{})
	return func(pointer string) {
		trimmed := strings.TrimSpace(pointer)
		if trimmed == "" {
			return
		}
		if _, exists := seen[trimmed]; exists {
			return
		}
		seen[trimmed] = struct{}{}
		*target = append(*target, trimmed)
	}
}

func parseExpression(raw string) (expressionNode, error) {
	tokens, err := tokenize(raw)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("identity template expression must not be empty")
	}

	if len(tokens) == 1 {
		if pointer, ok, err := singleTokenPointer(tokens[0]); ok {
			if err != nil {
				return nil, err
			}
			return pointerNode{pointer: pointer}, nil
		}
		if tokens[0].quoted {
			return literalNode{value: tokens[0].text}, nil
		}
		return nil, fmt.Errorf("identity template expression %q is not supported", raw)
	}

	name := normalizeHelperName(tokens[0].text)
	if !isSupportedHelper(name) {
		return nil, fmt.Errorf("identity template helper %q is not supported", tokens[0].text)
	}

	args := make([]expressionNode, 0, len(tokens)-1)
	for _, item := range tokens[1:] {
		if pointer, ok, err := singleTokenPointer(item); ok {
			if err != nil {
				return nil, err
			}
			args = append(args, pointerNode{pointer: pointer})
			continue
		}
		args = append(args, literalNode{value: item.text})
	}

	if err := validateHelperArity(name, len(args)); err != nil {
		return nil, err
	}

	return helperNode{name: name, args: args}, nil
}

func tokenize(raw string) ([]token, error) {
	tokens := make([]token, 0, 4)
	var current strings.Builder
	var quote rune
	quoted := false
	escaped := false

	flush := func() {
		if current.Len() == 0 {
			return
		}
		tokens = append(tokens, token{text: current.String(), quoted: quoted})
		current.Reset()
		quoted = false
	}

	for _, r := range raw {
		if quote != 0 {
			if escaped {
				current.WriteRune(r)
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
				continue
			}
			current.WriteRune(r)
			continue
		}

		switch {
		case r == '"' || r == '\'':
			quote = r
			quoted = true
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		default:
			current.WriteRune(r)
		}
	}

	if escaped || quote != 0 {
		return nil, fmt.Errorf("identity template contains an unterminated quoted string")
	}
	flush()
	return tokens, nil
}

func normalizeHelperName(name string) string {
	switch strings.TrimSpace(name) {
	case "uppercase", "to_uppercase":
		return "uppercase"
	case "lowercase", "to_lowercase":
		return "lowercase"
	default:
		return strings.TrimSpace(name)
	}
}

func isSupportedHelper(name string) bool {
	switch name {
	case "uppercase", "lowercase", "trim", "substring", "default":
		return true
	default:
		return false
	}
}

func validateHelperArity(name string, count int) error {
	switch name {
	case "uppercase", "lowercase", "trim":
		if count != 1 {
			return fmt.Errorf("identity template helper %q expects exactly 1 argument", name)
		}
	case "substring":
		if count != 2 && count != 3 {
			return fmt.Errorf("identity template helper %q expects 2 or 3 arguments", name)
		}
	case "default":
		if count < 2 {
			return fmt.Errorf("identity template helper %q expects at least 2 arguments", name)
		}
	}
	return nil
}

func singleTokenPointer(item token) (string, bool, error) {
	if item.quoted {
		return "", false, nil
	}

	trimmed := strings.TrimSpace(item.text)
	if strings.HasPrefix(trimmed, "/") {
		if _, err := resource.ParseJSONPointer(trimmed); err != nil {
			return "", false, err
		}
		return trimmed, true, nil
	}

	if barePointerIdentifierPattern.MatchString(trimmed) {
		return resource.JSONPointerForObjectKey(trimmed), true, nil
	}

	return "", false, nil
}

func (p literalPart) render(_ any) (string, error) {
	return p.text, nil
}

func (p literalPart) collectPointers(add func(string)) {
	_ = add
}

func (p expressionPart) render(payload any) (string, error) {
	value, missing, err := p.node.eval(payload)
	if err != nil {
		return "", fmt.Errorf("identity template expression %q failed: %w", p.raw, err)
	}
	if missing {
		return "", fmt.Errorf("identity template expression %q did not resolve to a value", p.raw)
	}
	return value, nil
}

func (p expressionPart) collectPointers(add func(string)) {
	p.node.collectPointers(add)
}

func (n literalNode) eval(_ any) (string, bool, error) {
	return n.value, strings.TrimSpace(n.value) == "", nil
}

func (n literalNode) collectPointers(add func(string)) {
	_ = add
}

func (n pointerNode) eval(payload any) (string, bool, error) {
	value, found, err := resource.LookupJSONPointer(payload, n.pointer)
	if err != nil {
		return "", false, err
	}
	if !found || value == nil {
		return "", true, nil
	}

	text, ok := scalarString(value)
	if !ok {
		return "", false, fmt.Errorf("JSON pointer %q resolved to a non-scalar value", n.pointer)
	}
	if strings.TrimSpace(text) == "" {
		return "", true, nil
	}
	return text, false, nil
}

func (n pointerNode) collectPointers(add func(string)) {
	add(n.pointer)
}

func (n helperNode) eval(payload any) (string, bool, error) {
	switch n.name {
	case "uppercase":
		value, err := evalRequiredStringArg(n.name, 0, n.args[0], payload)
		if err != nil {
			return "", false, err
		}
		return strings.ToUpper(value), false, nil
	case "lowercase":
		value, err := evalRequiredStringArg(n.name, 0, n.args[0], payload)
		if err != nil {
			return "", false, err
		}
		return strings.ToLower(value), false, nil
	case "trim":
		value, err := evalRequiredStringArg(n.name, 0, n.args[0], payload)
		if err != nil {
			return "", false, err
		}
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return "", true, nil
		}
		return trimmed, false, nil
	case "substring":
		value, err := evalRequiredStringArg(n.name, 0, n.args[0], payload)
		if err != nil {
			return "", false, err
		}
		start, err := evalRequiredIntArg(n.name, 1, n.args[1], payload)
		if err != nil {
			return "", false, err
		}
		runes := []rune(value)
		if start < 0 || start > len(runes) {
			return "", false, fmt.Errorf("helper %q start index %d is out of range", n.name, start)
		}

		end := len(runes)
		if len(n.args) == 3 {
			length, lengthErr := evalRequiredIntArg(n.name, 2, n.args[2], payload)
			if lengthErr != nil {
				return "", false, lengthErr
			}
			if length < 0 {
				return "", false, fmt.Errorf("helper %q length %d must not be negative", n.name, length)
			}
			end = start + length
			if end > len(runes) {
				return "", false, fmt.Errorf("helper %q range [%d:%d] is out of bounds", n.name, start, end)
			}
		}

		rendered := string(runes[start:end])
		if strings.TrimSpace(rendered) == "" {
			return "", true, nil
		}
		return rendered, false, nil
	case "default":
		for _, arg := range n.args {
			value, missing, err := arg.eval(payload)
			if err != nil {
				return "", false, err
			}
			if missing || strings.TrimSpace(value) == "" {
				continue
			}
			return value, false, nil
		}
		return "", true, nil
	default:
		return "", false, fmt.Errorf("identity template helper %q is not supported", n.name)
	}
}

func (n helperNode) collectPointers(add func(string)) {
	for _, arg := range n.args {
		arg.collectPointers(add)
	}
}

func evalRequiredStringArg(helper string, index int, arg expressionNode, payload any) (string, error) {
	value, missing, err := arg.eval(payload)
	if err != nil {
		return "", err
	}
	if missing || strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("helper %q argument %d did not resolve to a value", helper, index+1)
	}
	return value, nil
}

func evalRequiredIntArg(helper string, index int, arg expressionNode, payload any) (int, error) {
	value, err := evalRequiredStringArg(helper, index, arg, payload)
	if err != nil {
		return 0, err
	}

	parsed, parseErr := strconv.Atoi(value)
	if parseErr != nil {
		return 0, fmt.Errorf("helper %q argument %d must be an integer", helper, index+1)
	}
	return parsed, nil
}

func scalarString(value any) (string, bool) {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed), true
	case fmt.Stringer:
		return strings.TrimSpace(typed.String()), true
	case int:
		return strconv.Itoa(typed), true
	case int8:
		return strconv.FormatInt(int64(typed), 10), true
	case int16:
		return strconv.FormatInt(int64(typed), 10), true
	case int32:
		return strconv.FormatInt(int64(typed), 10), true
	case int64:
		return strconv.FormatInt(typed, 10), true
	case uint:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint8:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint16:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint32:
		return strconv.FormatUint(uint64(typed), 10), true
	case uint64:
		return strconv.FormatUint(typed, 10), true
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32), true
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64), true
	case bool:
		return strconv.FormatBool(typed), true
	default:
		return "", false
	}
}
