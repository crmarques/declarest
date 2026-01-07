package context

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	ConfigDirEnvVar  = "DECLAREST_CONFIG_DIR"
	ConfigFileEnvVar = "DECLAREST_CONFIG_FILE"
)

type ConfigPathInfo struct {
	Path    string
	FromEnv bool
}

// ConfigDirPathInfo returns the directory where DeclaREST stores context configurations,
// honoring DECLAREST_CONFIG_DIR when it is set.
func ConfigDirPathInfo() (ConfigPathInfo, error) {
	if dir, ok := configDirEnvValue(); ok {
		return ConfigPathInfo{Path: dir, FromEnv: true}, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ConfigPathInfo{}, fmt.Errorf("unable to determine home directory: %w", err)
	}
	return ConfigPathInfo{Path: filepath.Join(home, defaultConfigDir), FromEnv: false}, nil
}

// ConfigFilePathInfo returns the path to the context store file, honoring DECLAREST_CONFIG_FILE
// or constructing the default inside the chosen config directory.
func ConfigFilePathInfo() (ConfigPathInfo, error) {
	if file, ok := configFileEnvValue(); ok {
		return ConfigPathInfo{Path: file, FromEnv: true}, nil
	}
	dirInfo, err := ConfigDirPathInfo()
	if err != nil {
		return ConfigPathInfo{}, err
	}
	return ConfigPathInfo{Path: filepath.Join(dirInfo.Path, defaultConfigFile), FromEnv: false}, nil
}

func configDirEnvValue() (string, bool) {
	value, ok := os.LookupEnv(ConfigDirEnvVar)
	if !ok {
		return "", false
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}

func configFileEnvValue() (string, bool) {
	value, ok := os.LookupEnv(ConfigFileEnvVar)
	if !ok {
		return "", false
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", false
	}
	return trimmed, true
}
