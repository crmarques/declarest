package controllers

import (
	"fmt"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
)

func populateMetadataConfig(metadataPath string, resolvedContext *config.Context) error {
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
