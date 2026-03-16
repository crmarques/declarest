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

package controllers

import (
	"fmt"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
)

func populateMetadataConfigWithBundle(metadataPath string, metadataBundle string, resolvedContext *config.Context) error {
	metadataBundle = strings.TrimSpace(metadataBundle)
	if metadataBundle != "" {
		resolvedContext.Metadata.Bundle = metadataBundle
		return nil
	}

	metadataPath = strings.TrimSpace(metadataPath)
	if metadataPath == "" {
		return nil
	}
	if strings.HasSuffix(metadataPath, ".tar.gz") || strings.HasSuffix(metadataPath, ".tgz") {
		resolvedContext.Metadata.Bundle = metadataPath
		return nil
	}
	info, err := os.Stat(metadataPath)
	if err != nil {
		return fmt.Errorf("resolve metadata artifact %q: %w", metadataPath, err)
	}
	if info.IsDir() {
		resolvedContext.Metadata.BaseDir = metadataPath
		return nil
	}
	return fmt.Errorf("metadata artifact %q must be a .tar.gz bundle or directory", metadataPath)
}
