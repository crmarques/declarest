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
	repoBaseDir := normalizeContextBaseDir(ContextRepositoryBaseDir(cfg))
	metadataBaseDir := normalizeContextBaseDir(cfg.Metadata.BaseDir)
	return repoBaseDir != "" && metadataBaseDir != "" && repoBaseDir == metadataBaseDir
}

func ContextRepositoryBaseDir(cfg Context) string {
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
