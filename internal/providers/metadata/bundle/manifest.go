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

package bundlemetadata

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/crmarques/declarest/faults"
	"go.yaml.in/yaml/v3"
)

const (
	bundleManifestAPIVersion = "declarest.io/v1alpha1"
	bundleManifestKind       = "MetadataBundle"
)

var semverPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

var metadataFileCandidates = []string{"metadata.yaml", "metadata.yml", "metadata.json"}

type BundleManifest struct {
	APIVersion   string             `yaml:"apiVersion"`
	Kind         string             `yaml:"kind"`
	Name         string             `yaml:"name"`
	Version      string             `yaml:"version"`
	Description  string             `yaml:"description"`
	Deprecated   bool               `yaml:"deprecated,omitempty"`
	Declarest    BundleDeclarest    `yaml:"declarest"`
	Distribution BundleDistribution `yaml:"distribution,omitempty"`
}

type BundleDeclarest struct {
	MetadataRoot string `yaml:"metadataRoot"`
	OpenAPI      string `yaml:"openapi,omitempty"`
}

type BundleDistribution struct {
	Repo             string `yaml:"repo,omitempty"`
	TagTemplate      string `yaml:"tagTemplate,omitempty"`
	ArtifactTemplate string `yaml:"artifactTemplate,omitempty"`
}

func DecodeBundleManifest(data []byte) (BundleManifest, error) {
	var manifest BundleManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return BundleManifest{}, faults.NewValidationError("invalid bundle.yaml", err)
	}
	if err := manifest.Validate(); err != nil {
		return BundleManifest{}, err
	}
	return manifest, nil
}

func (m BundleManifest) Validate() error {
	if strings.TrimSpace(m.APIVersion) == "" {
		return faults.NewValidationError("bundle.yaml apiVersion is required", nil)
	}
	if strings.TrimSpace(m.Kind) == "" {
		return faults.NewValidationError("bundle.yaml kind is required", nil)
	}
	if strings.TrimSpace(m.Name) == "" {
		return faults.NewValidationError("bundle.yaml name is required", nil)
	}
	if strings.TrimSpace(m.Version) == "" {
		return faults.NewValidationError("bundle.yaml version is required", nil)
	}
	if strings.TrimSpace(m.Description) == "" {
		return faults.NewValidationError("bundle.yaml description is required", nil)
	}
	if strings.TrimSpace(m.Declarest.MetadataRoot) == "" {
		return faults.NewValidationError("bundle.yaml declarest.metadataRoot is required", nil)
	}

	if strings.TrimSpace(m.APIVersion) != bundleManifestAPIVersion {
		return faults.NewValidationError(
			fmt.Sprintf("bundle.yaml apiVersion must be %q", bundleManifestAPIVersion),
			nil,
		)
	}
	if strings.TrimSpace(m.Kind) != bundleManifestKind {
		return faults.NewValidationError(
			fmt.Sprintf("bundle.yaml kind must be %q", bundleManifestKind),
			nil,
		)
	}

	if _, err := normalizeSemver(m.Version); err != nil {
		return faults.NewValidationError("bundle.yaml version must be a valid semver", err)
	}

	if _, err := normalizeBundleRelativePath(m.Declarest.MetadataRoot); err != nil {
		return faults.NewValidationError("bundle.yaml declarest.metadataRoot is invalid", err)
	}

	if openAPIRef := strings.TrimSpace(m.Declarest.OpenAPI); openAPIRef != "" {
		if _, err := resolveBundleOpenAPIReference("", openAPIRef); err != nil {
			return faults.NewValidationError("bundle.yaml declarest.openapi is invalid", err)
		}
	}

	if template := strings.TrimSpace(m.Distribution.ArtifactTemplate); template != "" {
		expected := expectedArtifactTemplate(strings.TrimSpace(m.Name))
		if template != expected {
			return faults.NewValidationError(
				fmt.Sprintf(
					"bundle.yaml distribution.artifactTemplate must be %q",
					expected,
				),
				nil,
			)
		}
	}

	return nil
}

func (m BundleManifest) NormalizedMetadataRoot() (string, error) {
	return normalizeBundleRelativePath(m.Declarest.MetadataRoot)
}

func normalizeSemver(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if after, ok := strings.CutPrefix(value, "v"); ok {
		value = after
	}
	if !semverPattern.MatchString(value) {
		return "", faults.NewValidationError("invalid semver", nil)
	}
	return value, nil
}

func normalizeBundleRelativePath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", faults.NewValidationError("path is empty", nil)
	}

	cleaned := filepath.Clean(filepath.FromSlash(value))
	if cleaned == "." || cleaned == "" {
		return "", faults.NewValidationError("path is empty", nil)
	}
	if filepath.IsAbs(cleaned) {
		return "", faults.NewValidationError("path must be relative", nil)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", faults.NewValidationError("path must not traverse parents", nil)
	}
	return cleaned, nil
}

func expectedArtifactTemplate(name string) string {
	return fmt.Sprintf("%s-{version}.tar.gz", strings.TrimSpace(name))
}

func resolveBundleOpenAPIReference(root string, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", nil
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return "", faults.NewValidationError("OpenAPI reference is invalid", err)
	}

	if parsed.Scheme == "" {
		relativePath, err := normalizeBundleRelativePath(value)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(root) == "" {
			return relativePath, nil
		}
		return filepath.Join(root, relativePath), nil
	}

	switch strings.ToLower(parsed.Scheme) {
	case "http", "https":
		if strings.TrimSpace(parsed.Host) == "" {
			return "", faults.NewValidationError("OpenAPI URL host is required", nil)
		}
		return parsed.String(), nil
	default:
		return "", faults.NewValidationError("OpenAPI reference must be a relative path or http/https URL", nil)
	}
}

func transportError(message string, cause error) error {
	return faults.NewTypedError(faults.TransportError, message, cause)
}

func notFoundError(message string, cause error) error {
	return faults.NewTypedError(faults.NotFoundError, message, cause)
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}
