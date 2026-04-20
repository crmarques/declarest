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
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/promptauth"
	"github.com/crmarques/declarest/internal/providers/fsutil"
	binaryversion "github.com/crmarques/declarest/internal/version"
)

type BundleResolution struct {
	MetadataDir       string
	OpenAPI           string
	Manifest          BundleManifest
	Shorthand         bool
	DeprecatedWarning string
}

type BundleResolverOption func(*bundleResolverOptions)

type bundleResolverOptions struct {
	proxy            *config.HTTPProxy
	runtime          *promptauth.Runtime
	declarestVersion string
}

func WithProxyConfig(proxy *config.HTTPProxy) BundleResolverOption {
	return func(opts *bundleResolverOptions) {
		opts.proxy = proxy
	}
}

func WithPromptRuntime(runtime *promptauth.Runtime) BundleResolverOption {
	return func(opts *bundleResolverOptions) {
		opts.runtime = runtime
	}
}

// WithDeclarestVersion sets the declarest binary version used to evaluate the
// bundle manifest's declarest.compatibleDeclarest constraint. The literal
// value `dev` (the unstamped development build identifier) bypasses the gate.
// When this option is omitted the resolver falls back to the linker-stamped
// internal/version.Version value.
func WithDeclarestVersion(value string) BundleResolverOption {
	return func(opts *bundleResolverOptions) {
		opts.declarestVersion = value
	}
}

func ResolveBundle(ctx context.Context, ref string, opts ...BundleResolverOption) (BundleResolution, error) {
	options := bundleResolverOptions{}
	for _, opt := range opts {
		if opt != nil {
			opt(&options)
		}
	}
	source, err := parseBundleSource(ref)
	if err != nil {
		return BundleResolution{}, err
	}

	cacheRoot, err := defaultCacheRoot()
	if err != nil {
		return BundleResolution{}, err
	}

	cacheDir := filepath.Join(cacheRoot, source.cacheDirName)
	releaseCacheLock := acquireBundleCacheLock(cacheDir)
	defer releaseCacheLock()

	resolution, err := resolveBundleAt(ctx, cacheRoot, cacheDir, source, options)
	if err != nil {
		return BundleResolution{}, err
	}

	if err := enforceCompatibleDeclarest(resolution.Manifest, options.declarestVersion); err != nil {
		return BundleResolution{}, err
	}

	return resolution, nil
}

func resolveBundleAt(
	ctx context.Context,
	cacheRoot string,
	cacheDir string,
	source bundleSource,
	options bundleResolverOptions,
) (BundleResolution, error) {
	if cached, ok, cachedErr := loadCachedBundle(cacheDir, source); cachedErr == nil && ok {
		return cached, nil
	} else if cachedErr != nil {
		_ = os.RemoveAll(cacheDir)
	}

	return installBundle(ctx, cacheRoot, cacheDir, source, options)
}

func enforceCompatibleDeclarest(manifest BundleManifest, configuredVersion string) error {
	constraintRaw := strings.TrimSpace(manifest.Declarest.CompatibleDeclarest)
	if constraintRaw == "" {
		return nil
	}

	binaryVersion := strings.TrimSpace(configuredVersion)
	if binaryVersion == "" {
		binaryVersion = strings.TrimSpace(binaryversion.Version)
	}
	if binaryVersion == "" || binaryVersion == binaryversion.DevBuild {
		return nil
	}

	normalized, err := normalizeSemver(binaryVersion)
	if err != nil {
		return faults.Invalid(
			fmt.Sprintf("declarest binary version %q is not a valid semver for compatibleDeclarest evaluation", binaryVersion),
			err,
		)
	}

	parsedVersion, err := semver.NewVersion(normalized)
	if err != nil {
		return faults.Invalid(
			fmt.Sprintf("declarest binary version %q is not a valid semver for compatibleDeclarest evaluation", binaryVersion),
			err,
		)
	}

	constraint, err := semver.NewConstraint(constraintRaw)
	if err != nil {
		return faults.Invalid("bundle.yaml declarest.compatibleDeclarest is not a valid semver constraint", err)
	}

	if !constraint.Check(parsedVersion) {
		return faults.Invalid(
			fmt.Sprintf(
				"bundle %q version %q requires declarest %q but running binary is %q",
				strings.TrimSpace(manifest.Name),
				strings.TrimSpace(manifest.Version),
				constraintRaw,
				binaryVersion,
			),
			nil,
		)
	}

	return nil
}

func readBundleManifest(path string) (BundleManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return BundleManifest{}, faults.NotFound("bundle.yaml not found in metadata bundle", err)
		}
		return BundleManifest{}, faults.Internal("failed to read bundle.yaml", err)
	}

	manifest, err := DecodeBundleManifest(data)
	if err != nil {
		return BundleManifest{}, err
	}
	return manifest, nil
}

func buildResolution(root string, manifest BundleManifest, source bundleSource) (BundleResolution, error) {
	if source.kind == sourceKindShort {
		if strings.TrimSpace(manifest.Name) != source.shorthandName {
			return BundleResolution{}, faults.Invalid("bundle name does not match shorthand target", nil)
		}

		bundleVersion, err := normalizeSemver(manifest.Version)
		if err != nil {
			return BundleResolution{}, faults.Invalid("bundle version is invalid", err)
		}
		if bundleVersion != source.shorthandVersion {
			return BundleResolution{}, faults.Invalid("bundle version does not match shorthand version", nil)
		}

		if strings.TrimSpace(manifest.Distribution.ArtifactTemplate) == "" {
			return BundleResolution{}, faults.Invalid("bundle shorthand requires distribution.artifactTemplate", nil)
		}
	}

	metadataRoot, err := manifest.NormalizedMetadataRoot()
	if err != nil {
		return BundleResolution{}, err
	}

	metadataDir := filepath.Join(root, metadataRoot)
	if !fsutil.IsPathUnderRoot(root, metadataDir) {
		return BundleResolution{}, faults.Invalid("bundle metadata root escapes extracted bundle directory", nil)
	}

	info, err := os.Stat(metadataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return BundleResolution{}, faults.Invalid("bundle metadata root does not exist", nil)
		}
		return BundleResolution{}, faults.Internal("failed to inspect bundle metadata root", err)
	}
	if !info.IsDir() {
		return BundleResolution{}, faults.Invalid("bundle metadata root is not a directory", nil)
	}

	if err := ensureMetadataTreeHasDefinition(metadataDir, metadataFileCandidates); err != nil {
		return BundleResolution{}, err
	}

	openAPISource, err := resolveBundleOpenAPISource(root, metadataRoot, manifest)
	if err != nil {
		return BundleResolution{}, err
	}

	resolution := BundleResolution{
		MetadataDir: metadataDir,
		OpenAPI:     openAPISource,
		Manifest:    manifest,
		Shorthand:   source.kind == sourceKindShort,
	}
	if manifest.Deprecated {
		resolution.DeprecatedWarning = fmt.Sprintf(
			"metadata bundle %q version %q is deprecated",
			manifest.Name,
			manifest.Version,
		)
	}

	return resolution, nil
}

func resolveBundleOpenAPISource(root string, metadataRoot string, manifest BundleManifest) (string, error) {
	configuredRef := strings.TrimSpace(manifest.Declarest.OpenAPI)
	if configuredRef != "" {
		value, err := resolveBundleOpenAPIReference(root, configuredRef)
		if err != nil {
			return "", faults.Invalid("bundle declarest.openapi is invalid", err)
		}

		parsed, parseErr := url.Parse(value)
		if parseErr == nil && parsed.Scheme != "" {
			return value, nil
		}

		if err := ensureBundleFilePath(root, value, "bundle declarest.openapi"); err != nil {
			return "", err
		}
		return value, nil
	}

	bundledPath, err := resolveBundledOpenAPIFile(root, metadataRoot)
	if err != nil {
		return "", err
	}
	return bundledPath, nil
}

func resolveBundledOpenAPIFile(root string, metadataRoot string) (string, error) {
	openAPIFileNames := []string{"openapi.yaml", "openapi.yml", "openapi.json"}

	checkedPaths := map[string]struct{}{}
	priorityCandidates := make([]string, 0, len(openAPIFileNames)*2)
	for _, fileName := range openAPIFileNames {
		priorityCandidates = append(priorityCandidates, filepath.Join(root, fileName))
	}
	trimmedMetadataRoot := strings.TrimSpace(metadataRoot)
	if trimmedMetadataRoot != "" {
		for _, fileName := range openAPIFileNames {
			priorityCandidates = append(priorityCandidates, filepath.Join(root, trimmedMetadataRoot, fileName))
		}
	}

	for _, candidate := range priorityCandidates {
		normalizedCandidate := filepath.Clean(candidate)
		if _, seen := checkedPaths[normalizedCandidate]; seen {
			continue
		}
		checkedPaths[normalizedCandidate] = struct{}{}

		normalizedPath, ok, err := bundledOpenAPIFilePath(root, normalizedCandidate)
		if err != nil {
			return "", err
		}
		if ok {
			return normalizedPath, nil
		}
	}

	allowedNames := map[string]struct{}{
		"openapi.yaml": {},
		"openapi.yml":  {},
		"openapi.json": {},
	}
	recursiveCandidates := make([]string, 0, 1)
	walkErr := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		if _, ok := allowedNames[strings.ToLower(strings.TrimSpace(entry.Name()))]; !ok {
			return nil
		}

		recursiveCandidates = append(recursiveCandidates, path)
		return nil
	})
	if walkErr != nil {
		return "", faults.Internal("failed to inspect bundled OpenAPI fallback files", walkErr)
	}

	sort.Strings(recursiveCandidates)
	for _, candidate := range recursiveCandidates {
		normalizedCandidate := filepath.Clean(candidate)
		if _, seen := checkedPaths[normalizedCandidate]; seen {
			continue
		}
		checkedPaths[normalizedCandidate] = struct{}{}

		normalizedPath, ok, err := bundledOpenAPIFilePath(root, normalizedCandidate)
		if err != nil {
			return "", err
		}
		if ok {
			return normalizedPath, nil
		}
	}

	return "", nil
}

func bundledOpenAPIFilePath(root string, candidate string) (string, bool, error) {
	if !fsutil.IsPathUnderRoot(root, candidate) {
		return "", false, faults.Invalid("bundled openapi candidate escapes extracted bundle directory", nil)
	}

	info, err := os.Stat(candidate)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, faults.Internal("failed to inspect bundled openapi candidate", err)
	}
	if info.IsDir() {
		return "", false, nil
	}
	return candidate, true, nil
}
