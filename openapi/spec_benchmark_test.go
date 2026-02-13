package openapi

import (
	"encoding/json"
	"fmt"
	"testing"
)

func BenchmarkSpecMatchPath(b *testing.B) {
	spec := benchmarkSpec(b, 1500)
	target := "/api/v1/resource-1200/123"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = spec.MatchPath(target)
	}
}

func benchmarkSpec(b *testing.B, size int) *Spec {
	b.Helper()

	paths := make(map[string]any, size*2)
	for i := 0; i < size; i++ {
		base := fmt.Sprintf("/api/v1/resource-%d", i)
		paths[base] = benchmarkOperation()
		paths[base+"/{id}"] = benchmarkOperation()
	}

	doc := map[string]any{
		"openapi": "3.0.0",
		"paths":   paths,
	}

	data, err := json.Marshal(doc)
	if err != nil {
		b.Fatalf("marshal spec: %v", err)
	}
	spec, err := ParseSpec(data)
	if err != nil {
		b.Fatalf("parse spec: %v", err)
	}
	return spec
}

func benchmarkOperation() map[string]any {
	return map[string]any{
		"get": map[string]any{
			"responses": map[string]any{
				"200": map[string]any{
					"description": "ok",
				},
			},
		},
	}
}
