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
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/crmarques/declarest/faults"
	"go.yaml.in/yaml/v3"
)

const (
	bundleManifestAPIVersion = "declarest.io/v1alpha1"
	bundleManifestKind       = "MetadataBundle"
)

var (
	semverPattern              = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)
	managedServiceProductRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
)

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
	MetadataRoot             string                         `yaml:"metadataRoot"`
	OpenAPI                  string                         `yaml:"openapi,omitempty"`
	CompatibleDeclarest      string                         `yaml:"compatibleDeclarest,omitempty"`
	CompatibleManagedService BundleCompatibleManagedService `yaml:"compatibleManagedService,omitempty"`
}

type BundleCompatibleManagedService struct {
	Product  string `yaml:"product,omitempty"`
	Versions string `yaml:"versions,omitempty"`
}

type BundleDistribution struct {
	ArtifactTemplate string `yaml:"artifactTemplate,omitempty"`
}

func DecodeBundleManifest(data []byte) (BundleManifest, error) {
	var manifest BundleManifest
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&manifest); err != nil {
		return BundleManifest{}, faults.Invalid("invalid bundle.yaml", err)
	}
	if err := manifest.Validate(); err != nil {
		return BundleManifest{}, err
	}
	return manifest, nil
}

func (m BundleManifest) Validate() error {
	if strings.TrimSpace(m.APIVersion) == "" {
		return faults.Invalid("bundle.yaml apiVersion is required", nil)
	}
	if strings.TrimSpace(m.Kind) == "" {
		return faults.Invalid("bundle.yaml kind is required", nil)
	}
	if strings.TrimSpace(m.Name) == "" {
		return faults.Invalid("bundle.yaml name is required", nil)
	}
	if strings.TrimSpace(m.Version) == "" {
		return faults.Invalid("bundle.yaml version is required", nil)
	}
	if strings.TrimSpace(m.Description) == "" {
		return faults.Invalid("bundle.yaml description is required", nil)
	}
	if strings.TrimSpace(m.Declarest.MetadataRoot) == "" {
		return faults.Invalid("bundle.yaml declarest.metadataRoot is required", nil)
	}

	if strings.TrimSpace(m.APIVersion) != bundleManifestAPIVersion {
		return faults.Invalid(
			fmt.Sprintf("bundle.yaml apiVersion must be %q", bundleManifestAPIVersion),
			nil,
		)
	}
	if strings.TrimSpace(m.Kind) != bundleManifestKind {
		return faults.Invalid(
			fmt.Sprintf("bundle.yaml kind must be %q", bundleManifestKind),
			nil,
		)
	}

	if _, err := normalizeSemver(m.Version); err != nil {
		return faults.Invalid("bundle.yaml version must be a valid semver", err)
	}

	if _, err := normalizeBundleRelativePath(m.Declarest.MetadataRoot); err != nil {
		return faults.Invalid("bundle.yaml declarest.metadataRoot is invalid", err)
	}

	if openAPIRef := strings.TrimSpace(m.Declarest.OpenAPI); openAPIRef != "" {
		if _, err := resolveBundleOpenAPIReference("", openAPIRef); err != nil {
			return faults.Invalid("bundle.yaml declarest.openapi is invalid", err)
		}
	}

	if compat := strings.TrimSpace(m.Declarest.CompatibleDeclarest); compat != "" {
		if _, err := semver.NewConstraint(compat); err != nil {
			return faults.Invalid("bundle.yaml declarest.compatibleDeclarest is not a valid semver constraint", err)
		}
	}

	if err := m.Declarest.CompatibleManagedService.validate(); err != nil {
		return err
	}

	if template := strings.TrimSpace(m.Distribution.ArtifactTemplate); template != "" {
		expected := expectedArtifactTemplate(strings.TrimSpace(m.Name))
		if template != expected {
			return faults.Invalid(
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

func (c BundleCompatibleManagedService) validate() error {
	product := strings.TrimSpace(c.Product)
	versions := strings.TrimSpace(c.Versions)
	if product == "" && versions == "" {
		return nil
	}
	if product == "" {
		return faults.Invalid("bundle.yaml declarest.compatibleManagedService.product is required when versions is set", nil)
	}
	if versions == "" {
		return faults.Invalid("bundle.yaml declarest.compatibleManagedService.versions is required when product is set", nil)
	}
	if !managedServiceProductRegex.MatchString(product) {
		return faults.Invalid("bundle.yaml declarest.compatibleManagedService.product must match ^[a-z0-9][a-z0-9-]*$", nil)
	}
	if _, err := semver.NewConstraint(versions); err != nil {
		return faults.Invalid("bundle.yaml declarest.compatibleManagedService.versions is not a valid semver constraint", err)
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
		return "", faults.Invalid("invalid semver", nil)
	}
	return value, nil
}

func normalizeBundleRelativePath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", faults.Invalid("path is empty", nil)
	}

	cleaned := filepath.Clean(filepath.FromSlash(value))
	if cleaned == "." || cleaned == "" {
		return "", faults.Invalid("path is empty", nil)
	}
	if filepath.IsAbs(cleaned) {
		return "", faults.Invalid("path must be relative", nil)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", faults.Invalid("path must not traverse parents", nil)
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
		return "", faults.Invalid("OpenAPI reference is invalid", err)
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
			return "", faults.Invalid("OpenAPI URL host is required", nil)
		}
		return parsed.String(), nil
	default:
		return "", faults.Invalid("OpenAPI reference must be a relative path or http/https URL", nil)
	}
}
