package resource

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type ResourceKind uint8

const (
	KindNull ResourceKind = iota
	KindObject
	KindArray
	KindString
	KindNumber
	KindBool
)

type Resource struct {
	V any
}

type ResourceRecord struct {
	Path string
	Data Resource
	Meta ResourceMetadata
}

func (r ResourceRecord) CollectionPath() string {
	if r.Meta.ResourceInfo != nil {
		if path := strings.TrimSpace(r.Meta.ResourceInfo.CollectionPath); path != "" {
			return NormalizePath(path)
		}
	}

	base := strings.TrimSpace(r.Path)
	if base == "" {
		return "/"
	}

	isCollection := IsCollectionPath(base)
	base = NormalizePath(base)
	if isCollection {
		return base
	}

	last := LastSegment(base)
	if last == "" {
		return "/"
	}

	return NormalizePath(strings.TrimSuffix(base, "/"+last))
}

func NewResource(v any) (Resource, error) {
	nv, err := normalizeJSON(v)
	if err != nil {
		return Resource{}, err
	}
	return Resource{V: nv}, nil
}

func NewResourceFromJSON(b []byte) (Resource, error) {
	var r Resource
	if err := r.UnmarshalJSON(b); err != nil {
		return Resource{}, err
	}
	return r, nil
}

func (r *Resource) UnmarshalJSON(b []byte) error {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return err
	}
	nv, err := normalizeJSON(v)
	if err != nil {
		return err
	}
	r.V = nv
	return nil
}

func (r Resource) MarshalJSON() ([]byte, error) {
	if r.V == nil {
		return []byte("null"), nil
	}
	return json.Marshal(r.V)
}

func (r Resource) ContentType() string {
	switch r.Kind() {
	case KindNull, KindObject, KindArray, KindString, KindNumber, KindBool:
		return "application/json"
	default:
		return "application/octet-stream"
	}
}

func (r Resource) Kind() ResourceKind {
	switch r.V.(type) {
	case nil:
		return KindNull
	case map[string]any:
		return KindObject
	case []any:
		return KindArray
	case string:
		return KindString
	case json.Number:
		return KindNumber
	case bool:
		return KindBool
	default:
		return KindNull
	}
}

func (r Resource) AsObject() (map[string]any, bool) { v, ok := r.V.(map[string]any); return v, ok }
func (r Resource) AsArray() ([]any, bool)           { v, ok := r.V.([]any); return v, ok }
func (r Resource) AsString() (string, bool)         { v, ok := r.V.(string); return v, ok }
func (r Resource) AsBool() (bool, bool)             { v, ok := r.V.(bool); return v, ok }
func (r Resource) AsNumber() (json.Number, bool)    { v, ok := r.V.(json.Number); return v, ok }

func (r Resource) Clone() Resource {
	return Resource{V: deepCopyJSON(r.V)}
}

func normalizeJSON(v any) (any, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case bool, string, json.Number:
		return t, nil
	case float32, float64, int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64:
		return json.Number(fmt.Sprintf("%v", t)), nil
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, vv := range t {
			nv, err := normalizeJSON(vv)
			if err != nil {
				return nil, err
			}
			m[k] = nv
		}
		return m, nil
	case []any:
		s := make([]any, len(t))
		for i := range t {
			nv, err := normalizeJSON(t[i])
			if err != nil {
				return nil, err
			}
			s[i] = nv
		}
		return s, nil
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return nil, fmt.Errorf("value not JSON-serializable: %T: %w", t, err)
		}
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		var v2 any
		if err := dec.Decode(&v2); err != nil {
			return nil, err
		}
		return v2, nil
	}
}

func deepCopyJSON(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, vv := range t {
			m[k] = deepCopyJSON(vv)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i := range t {
			s[i] = deepCopyJSON(t[i])
		}
		return s
	case json.Number, string, bool:
		return t
	default:
		b, _ := json.Marshal(t)
		dec := json.NewDecoder(bytes.NewReader(b))
		dec.UseNumber()
		var v2 any
		if err := dec.Decode(&v2); err != nil {
			return nil
		}
		return v2
	}
}
