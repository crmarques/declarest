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

package file

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"go.yaml.in/yaml/v3"
)

func decodeCatalogFile(path string) (config.ContextCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return config.ContextCatalog{}, err
	}
	return decodeCatalog(data)
}

func decodeCatalog(data []byte) (config.ContextCatalog, error) {
	var contextCatalog config.ContextCatalog
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&contextCatalog); err != nil {
		return config.ContextCatalog{}, faults.NewValidationError("invalid context catalog yaml", err)
	}

	return contextCatalog, nil
}

func encodeCatalog(contextCatalog config.ContextCatalog) ([]byte, error) {
	data, err := yaml.Marshal(contextCatalog)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func resolveCatalogPath(explicitPath string) (string, error) {
	path := explicitPath
	if path == "" {
		path = os.Getenv(config.ContextFileEnvVar)
	}
	if path == "" {
		path = config.DefaultContextCatalogPath
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", internalError("failed to resolve user home directory", err)
	}

	if path == "~" {
		path = homeDir
	} else if strings.HasPrefix(path, "~/") {
		path = filepath.Join(homeDir, strings.TrimPrefix(path, "~/"))
	}

	if path == "" {
		return "", faults.NewValidationError("context catalog path is empty", nil)
	}

	cleanPath := filepath.Clean(path)
	if cleanPath == "." {
		return "", faults.NewValidationError("context catalog path is invalid", errors.New("resolved to current directory"))
	}

	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(homeDir, cleanPath)
	}

	return cleanPath, nil
}

func unknownOverrideError(key string) error {
	return faults.NewValidationError(fmt.Sprintf("unknown override key %q", key), nil)
}
