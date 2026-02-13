package file

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/ctx"
	"gopkg.in/yaml.v3"
)

func decodeCatalogFile(path string) (ctx.Catalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ctx.Catalog{}, err
	}
	return decodeCatalog(data)
}

func decodeCatalog(data []byte) (ctx.Catalog, error) {
	var catalog ctx.Catalog

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&catalog); err != nil {
		return ctx.Catalog{}, validationError("invalid context catalog yaml", err)
	}

	return catalog, nil
}

func encodeCatalog(catalog ctx.Catalog) ([]byte, error) {
	data, err := yaml.Marshal(catalog)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func resolveCatalogPath(explicitPath string) (string, error) {
	path := explicitPath
	if path == "" {
		path = os.Getenv(ctx.ContextFileEnvVar)
	}
	if path == "" {
		path = ctx.DefaultCatalogPath
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
