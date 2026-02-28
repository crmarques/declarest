package bundlemetadata

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"go.yaml.in/yaml/v3"
)

const (
	bundleManifestAPIVersion = "declarest.io/v1alpha1"
	bundleManifestKind       = "MetadataBundle"
)

var semverPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

type BundleManifest struct {
	APIVersion  string             `yaml:"apiVersion"`
	Kind        string             `yaml:"kind"`
	Name        string             `yaml:"name"`
	Version     string             `yaml:"version"`
	Description string             `yaml:"description"`
	Home        string             `yaml:"home,omitempty"`
	Sources     []string           `yaml:"sources,omitempty"`
	Keywords    []string           `yaml:"keywords,omitempty"`
	Maintainers []BundleMaintainer `yaml:"maintainers,omitempty"`
	License     string             `yaml:"license,omitempty"`
	Deprecated  bool               `yaml:"deprecated,omitempty"`
	Icon        string             `yaml:"icon,omitempty"`
	Annotations map[string]string  `yaml:"annotations,omitempty"`

	Declarest    BundleDeclarest    `yaml:"declarest"`
	Distribution BundleDistribution `yaml:"distribution,omitempty"`
}

type BundleMaintainer struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email,omitempty"`
	URL   string `yaml:"url,omitempty"`
}

type BundleDeclarest struct {
	Shorthand                string                         `yaml:"shorthand"`
	MetadataRoot             string                         `yaml:"metadataRoot"`
	OpenAPI                  string                         `yaml:"openapi,omitempty"`
	MetadataFileName         string                         `yaml:"metadataFileName,omitempty"`
	ResourceFormat           string                         `yaml:"resourceFormat,omitempty"`
	CompatibleDeclarest      string                         `yaml:"compatibleDeclarest,omitempty"`
	CompatibleResourceServer BundleCompatibleResourceServer `yaml:"compatibleResourceServer,omitempty"`
}

type BundleCompatibleResourceServer struct {
	Product  string `yaml:"product,omitempty"`
	Versions string `yaml:"versions,omitempty"`
}

type BundleDistribution struct {
	Repo             string `yaml:"repo,omitempty"`
	TagTemplate      string `yaml:"tagTemplate,omitempty"`
	ArtifactTemplate string `yaml:"artifactTemplate,omitempty"`
}

func DecodeBundleManifest(data []byte) (BundleManifest, error) {
	var manifest BundleManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return BundleManifest{}, validationError("invalid bundle.yaml", err)
	}
	if err := manifest.Validate(); err != nil {
		return BundleManifest{}, err
	}
	return manifest, nil
}

func (m BundleManifest) Validate() error {
	if strings.TrimSpace(m.APIVersion) == "" {
		return validationError("bundle.yaml apiVersion is required", nil)
	}
	if strings.TrimSpace(m.Kind) == "" {
		return validationError("bundle.yaml kind is required", nil)
	}
	if strings.TrimSpace(m.Name) == "" {
		return validationError("bundle.yaml name is required", nil)
	}
	if strings.TrimSpace(m.Version) == "" {
		return validationError("bundle.yaml version is required", nil)
	}
	if strings.TrimSpace(m.Description) == "" {
		return validationError("bundle.yaml description is required", nil)
	}
	if strings.TrimSpace(m.Declarest.MetadataRoot) == "" {
		return validationError("bundle.yaml declarest.metadataRoot is required", nil)
	}
	if strings.TrimSpace(m.Declarest.Shorthand) == "" {
		return validationError("bundle.yaml declarest.shorthand is required", nil)
	}

	if strings.TrimSpace(m.APIVersion) != bundleManifestAPIVersion {
		return validationError(
			fmt.Sprintf("bundle.yaml apiVersion must be %q", bundleManifestAPIVersion),
			nil,
		)
	}
	if strings.TrimSpace(m.Kind) != bundleManifestKind {
		return validationError(
			fmt.Sprintf("bundle.yaml kind must be %q", bundleManifestKind),
			nil,
		)
	}

	if _, err := normalizeSemver(m.Version); err != nil {
		return validationError("bundle.yaml version must be a valid semver", err)
	}

	if strings.TrimSpace(m.Name) != strings.TrimSpace(m.Declarest.Shorthand) {
		return validationError("bundle.yaml name must match declarest.shorthand", nil)
	}

	if _, err := normalizeBundleRelativePath(m.Declarest.MetadataRoot); err != nil {
		return validationError("bundle.yaml declarest.metadataRoot is invalid", err)
	}

	if fileName := strings.TrimSpace(m.Declarest.MetadataFileName); fileName != "" {
		if strings.Contains(fileName, "/") || strings.Contains(fileName, string(filepath.Separator)) {
			return validationError("bundle.yaml declarest.metadataFileName must be a file name", nil)
		}
	}

	if format := strings.TrimSpace(m.Declarest.ResourceFormat); format != "" {
		if format != config.ResourceFormatJSON && format != config.ResourceFormatYAML {
			return validationError("bundle.yaml declarest.resourceFormat must be json or yaml", nil)
		}
	}

	if openAPIRef := strings.TrimSpace(m.Declarest.OpenAPI); openAPIRef != "" {
		if _, err := resolveBundleOpenAPIReference("", openAPIRef); err != nil {
			return validationError("bundle.yaml declarest.openapi is invalid", err)
		}
	}

	if template := strings.TrimSpace(m.Distribution.ArtifactTemplate); template != "" {
		expected := expectedArtifactTemplate(strings.TrimSpace(m.Name))
		if template != expected {
			return validationError(
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

func (m BundleManifest) MetadataFileNameOrDefault() string {
	value := strings.TrimSpace(m.Declarest.MetadataFileName)
	if value == "" {
		return "metadata.json"
	}
	return value
}

func normalizeSemver(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if after, ok := strings.CutPrefix(value, "v"); ok {
		value = after
	}
	if !semverPattern.MatchString(value) {
		return "", validationError("invalid semver", nil)
	}
	return value, nil
}

func normalizeBundleRelativePath(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", validationError("path is empty", nil)
	}

	cleaned := filepath.Clean(filepath.FromSlash(value))
	if cleaned == "." || cleaned == "" {
		return "", validationError("path is empty", nil)
	}
	if filepath.IsAbs(cleaned) {
		return "", validationError("path must be relative", nil)
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", validationError("path must not traverse parents", nil)
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
		return "", validationError("OpenAPI reference is invalid", err)
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
			return "", validationError("OpenAPI URL host is required", nil)
		}
		return parsed.String(), nil
	default:
		return "", validationError("OpenAPI reference must be a relative path or http/https URL", nil)
	}
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
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
