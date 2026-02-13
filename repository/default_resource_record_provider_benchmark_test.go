package repository

import (
	"os"
	"path/filepath"
	"testing"
)

func BenchmarkDefaultResourceRecordProviderGetResourceRecord(b *testing.B) {
	dir := b.TempDir()
	writeMetadata := func(rel string, payload string) {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			b.Fatalf("mkdir metadata %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(payload), 0o644); err != nil {
			b.Fatalf("write metadata %s: %v", rel, err)
		}
	}

	writeMetadata("metadata.json", `{"resourceInfo":{"collectionPath":"/"}}`)
	writeMetadata(filepath.Join("a", "_", "metadata.json"), `{"resourceInfo":{"collectionPath":"/a"}}`)
	writeMetadata(filepath.Join("a", "_", "b", "_", "metadata.json"), `{"resourceInfo":{"collectionPath":"/a/b"}}`)
	writeMetadata(filepath.Join("a", "_", "b", "_", "c", "_", "metadata.json"), `{"resourceInfo":{"collectionPath":"/a/b/c"}}`)
	writeMetadata(filepath.Join("a", "_", "b", "_", "c", "_", "d", "_", "metadata.json"), `{"resourceInfo":{"collectionPath":"/a/b/c/d"}}`)
	writeMetadata(filepath.Join("a", "_", "b", "_", "c", "_", "d", "_", "e", "_", "metadata.json"), `{"resourceInfo":{"collectionPath":"/a/b/c/d/e"}}`)

	provider := NewDefaultResourceRecordProvider(dir, nil)
	target := "/a/b/c/d/e/f/g/h/item"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := provider.GetResourceRecord(target); err != nil {
			b.Fatalf("GetResourceRecord: %v", err)
		}
	}
}
