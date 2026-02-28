package bundlemetadata

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/crmarques/declarest/internal/providers/shared/fsutil"
)

const (
	defaultBundleOwner     = "crmarques"
	defaultBundleCacheDir  = ".declarest/metadata-bundles"
	bundleReadyMarkerFile  = ".declarest-bundle-ready"
	maxArchiveFileSizeByte = 64 << 20
)

const (
	sourceKindLocal = "local"
	sourceKindURL   = "url"
	sourceKindShort = "shorthand"
)

var shorthandNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)
var shorthandReleaseBaseURL = "https://github.com"

type BundleResolution struct {
	MetadataDir       string
	OpenAPI           string
	Manifest          BundleManifest
	Shorthand         bool
	DeprecatedWarning string
}

type bundleSource struct {
	kind             string
	cacheDirName     string
	localPath        string
	remoteURL        string
	shorthandName    string
	shorthandVersion string
}

func ResolveBundle(ctx context.Context, ref string) (BundleResolution, error) {
	source, err := parseBundleSource(ref)
	if err != nil {
		return BundleResolution{}, err
	}

	cacheRoot, err := defaultCacheRoot()
	if err != nil {
		return BundleResolution{}, err
	}

	cacheDir := filepath.Join(cacheRoot, source.cacheDirName)
	if cached, ok, cachedErr := loadCachedBundle(cacheDir, source); cachedErr == nil && ok {
		return cached, nil
	} else if cachedErr != nil {
		_ = os.RemoveAll(cacheDir)
	}

	return installBundle(ctx, cacheRoot, cacheDir, source)
}

func parseBundleSource(ref string) (bundleSource, error) {
	value := strings.TrimSpace(ref)
	if value == "" {
		return bundleSource{}, validationError("metadata.bundle is empty", nil)
	}

	if name, version, ok := parseShorthandRef(value); ok {
		repoName := shorthandRepositoryName(name)
		artifactName := shorthandArtifactName(name, version)
		baseURL := strings.TrimRight(strings.TrimSpace(shorthandReleaseBaseURL), "/")
		if baseURL == "" {
			baseURL = "https://github.com"
		}
		return bundleSource{
			kind:             sourceKindShort,
			shorthandName:    name,
			shorthandVersion: version,
			remoteURL: fmt.Sprintf(
				"%s/%s/%s/releases/download/v%s/%s",
				baseURL,
				defaultBundleOwner,
				repoName,
				version,
				artifactName,
			),
			cacheDirName: fmt.Sprintf("%s-%s", name, version),
		}, nil
	}

	if parsed, err := url.Parse(value); err == nil && parsed.Scheme != "" {
		switch strings.ToLower(parsed.Scheme) {
		case "http", "https":
			prefix := sanitizedCachePrefix(filepath.Base(parsed.Path))
			hashValue := shortStableHash("url:" + parsed.String())
			return bundleSource{
				kind:         sourceKindURL,
				remoteURL:    parsed.String(),
				cacheDirName: fmt.Sprintf("%s-%s", prefix, hashValue),
			}, nil
		default:
			return bundleSource{}, validationError("metadata.bundle URL must use http or https", nil)
		}
	}

	absolutePath, err := filepath.Abs(value)
	if err != nil {
		return bundleSource{}, validationError("metadata.bundle local path is invalid", err)
	}

	prefix := sanitizedCachePrefix(filepath.Base(absolutePath))
	hashValue := shortStableHash("local:" + absolutePath)
	return bundleSource{
		kind:         sourceKindLocal,
		localPath:    absolutePath,
		cacheDirName: fmt.Sprintf("%s-%s", prefix, hashValue),
	}, nil
}

func parseShorthandRef(value string) (name string, version string, ok bool) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	name = strings.TrimSpace(parts[0])
	versionRaw := strings.TrimSpace(parts[1])
	if name == "" || versionRaw == "" {
		return "", "", false
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", "", false
	}
	if !shorthandNamePattern.MatchString(name) {
		return "", "", false
	}

	normalizedVersion, err := normalizeSemver(versionRaw)
	if err != nil {
		return "", "", false
	}
	return name, normalizedVersion, true
}

func defaultCacheRoot() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", internalError("failed to resolve user home directory", err)
	}
	return filepath.Join(homeDir, defaultBundleCacheDir), nil
}

func loadCachedBundle(cacheDir string, source bundleSource) (BundleResolution, bool, error) {
	info, err := os.Stat(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return BundleResolution{}, false, nil
		}
		return BundleResolution{}, false, internalError("failed to inspect bundle cache directory", err)
	}
	if !info.IsDir() {
		return BundleResolution{}, false, validationError("bundle cache path is not a directory", nil)
	}

	if _, err := os.Stat(filepath.Join(cacheDir, bundleReadyMarkerFile)); err != nil {
		if os.IsNotExist(err) {
			return BundleResolution{}, false, nil
		}
		return BundleResolution{}, false, internalError("failed to inspect bundle cache readiness marker", err)
	}

	manifest, err := readBundleManifest(filepath.Join(cacheDir, "bundle.yaml"))
	if err != nil {
		return BundleResolution{}, false, err
	}

	resolution, err := buildResolution(cacheDir, manifest, source)
	if err != nil {
		return BundleResolution{}, false, err
	}

	return resolution, true, nil
}

func installBundle(ctx context.Context, cacheRoot string, cacheDir string, source bundleSource) (BundleResolution, error) {
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return BundleResolution{}, internalError("failed to create metadata bundle cache directory", err)
	}

	tmpDir, err := os.MkdirTemp(cacheRoot, source.cacheDirName+".tmp-")
	if err != nil {
		return BundleResolution{}, internalError("failed to create metadata bundle temporary directory", err)
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	if err := extractBundleArchive(ctx, source, tmpDir); err != nil {
		return BundleResolution{}, err
	}

	manifest, err := readBundleManifest(filepath.Join(tmpDir, "bundle.yaml"))
	if err != nil {
		return BundleResolution{}, err
	}

	if _, err := buildResolution(tmpDir, manifest, source); err != nil {
		return BundleResolution{}, err
	}

	if err := os.WriteFile(filepath.Join(tmpDir, bundleReadyMarkerFile), []byte("ok\n"), 0o600); err != nil {
		return BundleResolution{}, internalError("failed to write bundle cache readiness marker", err)
	}

	if err := os.RemoveAll(cacheDir); err != nil {
		return BundleResolution{}, internalError("failed to replace existing metadata bundle cache", err)
	}
	if err := os.Rename(tmpDir, cacheDir); err != nil {
		return BundleResolution{}, internalError("failed to finalize metadata bundle cache", err)
	}
	cleanupTmp = false

	return loadCachedOrFail(cacheDir, source)
}

func loadCachedOrFail(cacheDir string, source bundleSource) (BundleResolution, error) {
	resolution, ok, err := loadCachedBundle(cacheDir, source)
	if err != nil {
		return BundleResolution{}, err
	}
	if !ok {
		return BundleResolution{}, internalError("bundle cache was not ready after install", nil)
	}
	return resolution, nil
}

func readBundleManifest(path string) (BundleManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return BundleManifest{}, notFoundError("bundle.yaml not found in metadata bundle", err)
		}
		return BundleManifest{}, internalError("failed to read bundle.yaml", err)
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
			return BundleResolution{}, validationError("bundle name does not match shorthand target", nil)
		}

		bundleVersion, err := normalizeSemver(manifest.Version)
		if err != nil {
			return BundleResolution{}, validationError("bundle version is invalid", err)
		}
		if bundleVersion != source.shorthandVersion {
			return BundleResolution{}, validationError("bundle version does not match shorthand version", nil)
		}

		if strings.TrimSpace(manifest.Distribution.ArtifactTemplate) == "" {
			return BundleResolution{}, validationError("bundle shorthand requires distribution.artifactTemplate", nil)
		}
	}

	metadataRoot, err := manifest.NormalizedMetadataRoot()
	if err != nil {
		return BundleResolution{}, err
	}

	metadataDir := filepath.Join(root, metadataRoot)
	if !fsutil.IsPathUnderRoot(root, metadataDir) {
		return BundleResolution{}, validationError("bundle metadata root escapes extracted bundle directory", nil)
	}

	info, err := os.Stat(metadataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return BundleResolution{}, validationError("bundle metadata root does not exist", nil)
		}
		return BundleResolution{}, internalError("failed to inspect bundle metadata root", err)
	}
	if !info.IsDir() {
		return BundleResolution{}, validationError("bundle metadata root is not a directory", nil)
	}

	if err := ensureMetadataTreeHasDefinition(metadataDir, manifest.MetadataFileNameOrDefault()); err != nil {
		return BundleResolution{}, err
	}

	openAPISource, err := resolveBundleOpenAPISource(root, manifest)
	if err != nil {
		return BundleResolution{}, err
	}

	resolution := BundleResolution{
		MetadataDir: metadataDir,
		OpenAPI:     openAPISource,
		Manifest:    manifest,
		Shorthand:   source.kind == sourceKindShort,
	}
	if resolution.Shorthand && manifest.Deprecated {
		resolution.DeprecatedWarning = fmt.Sprintf(
			"metadata bundle %q version %q is deprecated",
			manifest.Name,
			manifest.Version,
		)
	}

	return resolution, nil
}

func resolveBundleOpenAPISource(root string, manifest BundleManifest) (string, error) {
	configuredRef := strings.TrimSpace(manifest.Declarest.OpenAPI)
	if configuredRef != "" {
		value, err := resolveBundleOpenAPIReference(root, configuredRef)
		if err != nil {
			return "", validationError("bundle declarest.openapi is invalid", err)
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

	peerPath := filepath.Join(root, "openapi.yaml")
	info, err := os.Stat(peerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", internalError("failed to inspect bundled openapi.yaml", err)
	}
	if info.IsDir() {
		return "", validationError("bundled openapi.yaml must be a file", nil)
	}
	if !fsutil.IsPathUnderRoot(root, peerPath) {
		return "", validationError("bundled openapi.yaml escapes extracted bundle directory", nil)
	}
	return peerPath, nil
}

func ensureBundleFilePath(root string, path string, field string) error {
	if !fsutil.IsPathUnderRoot(root, path) {
		return validationError(field+" escapes extracted bundle directory", nil)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return validationError(field+" file does not exist", nil)
		}
		return internalError("failed to inspect "+field+" file", err)
	}
	if info.IsDir() {
		return validationError(field+" must point to a file", nil)
	}
	return nil
}

func ensureMetadataTreeHasDefinition(metadataDir string, metadataFileName string) error {
	found := false
	walkErr := filepath.WalkDir(metadataDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(entry.Name(), metadataFileName) {
			found = true
			return io.EOF
		}
		return nil
	})
	if walkErr != nil && walkErr != io.EOF {
		return internalError("failed to inspect metadata root tree", walkErr)
	}
	if !found {
		return validationError(
			fmt.Sprintf("bundle metadata root does not contain %q", metadataFileName),
			nil,
		)
	}
	return nil
}

func extractBundleArchive(ctx context.Context, source bundleSource, destination string) error {
	stream, err := openBundleStream(ctx, source)
	if err != nil {
		return err
	}
	defer stream.Close()

	return extractTarGz(stream, destination)
}

func openBundleStream(ctx context.Context, source bundleSource) (io.ReadCloser, error) {
	switch source.kind {
	case sourceKindLocal:
		file, err := os.Open(source.localPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, notFoundError("local metadata bundle archive not found", err)
			}
			return nil, internalError("failed to open local metadata bundle archive", err)
		}
		return file, nil
	case sourceKindURL, sourceKindShort:
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.remoteURL, nil)
		if err != nil {
			return nil, validationError("metadata bundle URL is invalid", err)
		}

		response, err := (&http.Client{Timeout: 60 * time.Second}).Do(request)
		if err != nil {
			return nil, transportError("failed to download metadata bundle archive", err)
		}
		if response.StatusCode == http.StatusNotFound {
			_ = response.Body.Close()
			return nil, notFoundError("metadata bundle archive not found", nil)
		}
		if response.StatusCode >= http.StatusBadRequest {
			_ = response.Body.Close()
			return nil, transportError(
				fmt.Sprintf("metadata bundle download failed with status %d", response.StatusCode),
				nil,
			)
		}
		return response.Body, nil
	default:
		return nil, validationError("unsupported metadata bundle source", nil)
	}
}

func shorthandRepositoryName(name string) string {
	value := strings.TrimSpace(name)
	base := strings.TrimSuffix(value, "-bundle")
	if base == "" {
		base = value
	}
	return fmt.Sprintf("declarest-bundle-%s", base)
}

func shorthandArtifactName(name string, version string) string {
	return fmt.Sprintf("%s-%s.tar.gz", strings.TrimSpace(name), strings.TrimSpace(version))
}

func extractTarGz(stream io.Reader, destination string) error {
	gzipReader, err := gzip.NewReader(stream)
	if err != nil {
		return validationError("metadata bundle archive is not a valid gzip stream", err)
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return validationError("metadata bundle archive is not a valid tar stream", err)
		}

		entryName := strings.TrimSpace(header.Name)
		if entryName == "" {
			continue
		}
		entryPath := filepath.Clean(filepath.FromSlash(entryName))
		if entryPath == "." {
			continue
		}
		if filepath.IsAbs(entryPath) || entryPath == ".." || strings.HasPrefix(entryPath, ".."+string(filepath.Separator)) {
			return validationError("metadata bundle archive contains invalid path traversal entry", nil)
		}

		targetPath := filepath.Join(destination, entryPath)
		if !fsutil.IsPathUnderRoot(destination, targetPath) {
			return validationError("metadata bundle archive contains path outside extraction root", nil)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return internalError("failed to create bundle extraction directory", err)
			}
		case tar.TypeReg, tar.TypeRegA:
			if header.Size < 0 || header.Size > maxArchiveFileSizeByte {
				return validationError("metadata bundle archive contains oversized file entry", nil)
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return internalError("failed to create bundle extraction parent directory", err)
			}

			output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			if err != nil {
				return internalError("failed to create bundle extraction file", err)
			}

			limitedReader := io.LimitReader(tarReader, maxArchiveFileSizeByte+1)
			written, copyErr := io.Copy(output, limitedReader)
			closeErr := output.Close()
			if copyErr != nil {
				return internalError("failed to extract bundle archive file", copyErr)
			}
			if closeErr != nil {
				return internalError("failed to finalize bundle archive file", closeErr)
			}
			if written > maxArchiveFileSizeByte {
				return validationError("metadata bundle archive contains oversized file entry", nil)
			}
		default:
			return validationError("metadata bundle archive contains unsupported entry type", nil)
		}
	}
}

func sanitizedCachePrefix(raw string) string {
	prefix := strings.ToLower(strings.TrimSpace(raw))
	prefix = strings.TrimSuffix(prefix, ".tar.gz")
	prefix = strings.TrimSuffix(prefix, filepath.Ext(prefix))
	if prefix == "" {
		prefix = "bundle"
	}

	builder := strings.Builder{}
	for _, char := range prefix {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		default:
			builder.WriteByte('-')
		}
	}

	cleaned := strings.Trim(builder.String(), "-")
	if cleaned == "" {
		return "bundle"
	}
	return cleaned
}

func shortStableHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}
