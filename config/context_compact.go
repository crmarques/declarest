package config

import (
	"path/filepath"
	"strings"
)

// CompactContext removes deterministic default-equivalent values so callers can
// render or persist only explicit configuration.
func CompactContext(cfg Context) Context {
	if strings.TrimSpace(cfg.Metadata.Bundle) != "" || strings.TrimSpace(cfg.Metadata.BundleFile) != "" {
		cfg.Metadata.BaseDir = ""
		return cfg
	}
	if metadataUsesDefaultBaseDir(cfg) {
		cfg.Metadata.BaseDir = ""
	}
	return cfg
}

// CompactContextCatalog applies CompactContext to each catalog context.
func CompactContextCatalog(catalog ContextCatalog) ContextCatalog {
	if len(catalog.Contexts) == 0 {
		return catalog
	}

	compacted := catalog
	compacted.Contexts = make([]Context, len(catalog.Contexts))
	for idx, item := range catalog.Contexts {
		compacted.Contexts[idx] = CompactContext(item)
	}
	return compacted
}

func metadataUsesDefaultBaseDir(cfg Context) bool {
	repoBaseDir := normalizeContextBaseDir(contextRepositoryBaseDir(cfg))
	metadataBaseDir := normalizeContextBaseDir(cfg.Metadata.BaseDir)
	return repoBaseDir != "" && metadataBaseDir != "" && repoBaseDir == metadataBaseDir
}

func contextRepositoryBaseDir(cfg Context) string {
	switch {
	case cfg.Repository.Git != nil:
		return cfg.Repository.Git.Local.BaseDir
	case cfg.Repository.Filesystem != nil:
		return cfg.Repository.Filesystem.BaseDir
	default:
		return ""
	}
}

func normalizeContextBaseDir(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}
