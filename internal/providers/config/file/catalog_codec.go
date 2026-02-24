package file

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/config"
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
		return config.ContextCatalog{}, validationError("invalid context catalog yaml", err)
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
		return "", validationError("context catalog path is empty", nil)
	}

	cleanPath := filepath.Clean(path)
	if cleanPath == "." {
		return "", validationError("context catalog path is invalid", errors.New("resolved to current directory"))
	}

	if !filepath.IsAbs(cleanPath) {
		cleanPath = filepath.Join(homeDir, cleanPath)
	}

	return cleanPath, nil
}

func unknownOverrideError(key string) error {
	return validationError(fmt.Sprintf("unknown override key %q", key), nil)
}
