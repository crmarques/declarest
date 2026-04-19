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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/crmarques/declarest/faults"
)

const (
	defaultBundleOwner     = "crmarques"
	defaultBundleCacheDir  = ".declarest/metadata-bundles"
	bundleReadyMarkerFile  = ".declarest-bundle-ready"
	maxArchiveFileSizeByte = 64 << 20
	maxTotalArchiveBytes   = 256 << 20
	maxArchiveEntries      = 10_000
)

var (
	bundleCacheLockMu sync.Mutex
	bundleCacheLocks  = map[string]*bundleCacheLockEntry{}
)

type bundleCacheLockEntry struct {
	mu       sync.Mutex
	refCount int
}

func acquireBundleCacheLock(cacheDir string) func() {
	bundleCacheLockMu.Lock()
	lockEntry, ok := bundleCacheLocks[cacheDir]
	if !ok {
		lockEntry = &bundleCacheLockEntry{}
		bundleCacheLocks[cacheDir] = lockEntry
	}
	lockEntry.refCount++
	bundleCacheLockMu.Unlock()

	lockEntry.mu.Lock()

	return func() {
		lockEntry.mu.Unlock()

		bundleCacheLockMu.Lock()
		lockEntry.refCount--
		if lockEntry.refCount == 0 {
			delete(bundleCacheLocks, cacheDir)
		}
		bundleCacheLockMu.Unlock()
	}
}

func defaultCacheRoot() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", faults.Internal("failed to resolve user home directory", err)
	}
	return filepath.Join(homeDir, defaultBundleCacheDir), nil
}

func loadCachedBundle(cacheDir string, source bundleSource) (BundleResolution, bool, error) {
	info, err := os.Stat(cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return BundleResolution{}, false, nil
		}
		return BundleResolution{}, false, faults.Internal("failed to inspect bundle cache directory", err)
	}
	if !info.IsDir() {
		return BundleResolution{}, false, faults.Invalid("bundle cache path is not a directory", nil)
	}

	if _, err := os.Stat(filepath.Join(cacheDir, bundleReadyMarkerFile)); err != nil {
		if os.IsNotExist(err) {
			return BundleResolution{}, false, nil
		}
		return BundleResolution{}, false, faults.Internal("failed to inspect bundle cache readiness marker", err)
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

func installBundle(ctx context.Context, cacheRoot string, cacheDir string, source bundleSource, opts bundleResolverOptions) (BundleResolution, error) {
	if err := os.MkdirAll(cacheRoot, 0o755); err != nil {
		return BundleResolution{}, faults.Internal("failed to create metadata bundle cache directory", err)
	}

	tmpDir, err := os.MkdirTemp(cacheRoot, source.cacheDirName+".tmp-")
	if err != nil {
		return BundleResolution{}, faults.Internal("failed to create metadata bundle temporary directory", err)
	}
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	if err := extractBundleArchive(ctx, source, tmpDir, opts); err != nil {
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
		return BundleResolution{}, faults.Internal("failed to write bundle cache readiness marker", err)
	}

	oldDir := cacheDir + ".old"
	_ = os.RemoveAll(oldDir)

	if err := os.Rename(cacheDir, oldDir); err != nil && !os.IsNotExist(err) {
		return BundleResolution{}, faults.Internal("failed to move existing metadata bundle cache aside", err)
	}
	if err := os.Rename(tmpDir, cacheDir); err != nil {
		_ = os.Rename(oldDir, cacheDir)
		return BundleResolution{}, faults.Internal("failed to finalize metadata bundle cache", err)
	}
	_ = os.RemoveAll(oldDir)
	cleanupTmp = false

	return loadCachedOrFail(cacheDir, source)
}

func loadCachedOrFail(cacheDir string, source bundleSource) (BundleResolution, error) {
	resolution, ok, err := loadCachedBundle(cacheDir, source)
	if err != nil {
		return BundleResolution{}, err
	}
	if !ok {
		return BundleResolution{}, faults.Internal("bundle cache was not ready after install", nil)
	}
	return resolution, nil
}

func sanitizedCachePrefix(raw string) string {
	prefix := strings.ToLower(strings.TrimSpace(raw))
	prefix = strings.TrimSuffix(prefix, ".tar.gz")
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

func cacheDirNameForSourceArtifact(artifactName string, hashInput string) string {
	if name, version, ok := parseVersionedArtifactFileName(artifactName); ok {
		return fmt.Sprintf("%s-%s", name, version)
	}

	prefix := sanitizedCachePrefix(artifactName)
	hashValue := shortStableHash(hashInput)
	return fmt.Sprintf("%s-%s", prefix, hashValue)
}
