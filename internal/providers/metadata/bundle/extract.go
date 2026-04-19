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
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/httpclient"
	"github.com/crmarques/declarest/internal/providers/fsutil"
)

func ensureBundleFilePath(root string, path string, field string) error {
	if !fsutil.IsPathUnderRoot(root, path) {
		return faults.Invalid(field+" escapes extracted bundle directory", nil)
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return faults.Invalid(field+" file does not exist", nil)
		}
		return faults.Internal("failed to inspect "+field+" file", err)
	}
	if info.IsDir() {
		return faults.Invalid(field+" must point to a file", nil)
	}
	return nil
}

func ensureMetadataTreeHasDefinition(metadataDir string, candidates []string) error {
	found := false
	walkErr := filepath.WalkDir(metadataDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		for _, candidate := range candidates {
			if strings.EqualFold(entry.Name(), candidate) {
				found = true
				return io.EOF
			}
		}
		return nil
	})
	if walkErr != nil && walkErr != io.EOF {
		return faults.Internal("failed to inspect metadata root tree", walkErr)
	}
	if !found {
		return faults.Invalid(
			fmt.Sprintf("bundle metadata root does not contain any of %v", candidates),
			nil,
		)
	}
	return nil
}

func extractBundleArchive(ctx context.Context, source bundleSource, destination string, opts bundleResolverOptions) error {
	stream, err := openBundleStream(ctx, source, opts)
	if err != nil {
		return err
	}
	defer func() {
		_ = stream.Close()
	}()

	return extractTarGz(stream, destination)
}

func openBundleStream(ctx context.Context, source bundleSource, opts bundleResolverOptions) (io.ReadCloser, error) {
	switch source.kind {
	case sourceKindLocal:
		file, err := os.Open(source.localPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, faults.NotFound("local metadata bundle archive not found", err)
			}
			return nil, faults.Internal("failed to open local metadata bundle archive", err)
		}
		return file, nil
	case sourceKindURL, sourceKindShort:
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.remoteURL, nil)
		if err != nil {
			return nil, faults.Invalid("metadata bundle URL is invalid", err)
		}

		client, err := httpclient.Build(httpclient.Options{
			Timeout:      60 * time.Second,
			Proxy:        opts.proxy,
			ProxyScope:   "metadata.proxy",
			ProxyRuntime: opts.runtime,
		})
		if err != nil {
			return nil, err
		}

		response, err := client.Do(request)
		if err != nil {
			return nil, faults.Transport("failed to download metadata bundle archive", err)
		}
		if response.StatusCode == http.StatusNotFound {
			_ = response.Body.Close()
			return nil, faults.NotFound("metadata bundle archive not found", nil)
		}
		if response.StatusCode >= http.StatusBadRequest {
			_ = response.Body.Close()
			return nil, faults.Transport(
				fmt.Sprintf("metadata bundle download failed with status %d", response.StatusCode),
				nil,
			)
		}
		return response.Body, nil
	default:
		return nil, faults.Invalid("unsupported metadata bundle source", nil)
	}
}

func extractTarGz(stream io.Reader, destination string) error {
	gzipReader, err := gzip.NewReader(stream)
	if err != nil {
		return faults.Invalid("metadata bundle archive is not a valid gzip stream", err)
	}
	defer func() {
		_ = gzipReader.Close()
	}()

	tarReader := tar.NewReader(gzipReader)
	var totalBytes int64
	var entryCount int
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return faults.Invalid("metadata bundle archive is not a valid tar stream", err)
		}

		entryCount++
		if entryCount > maxArchiveEntries {
			return faults.Invalid("metadata bundle archive contains too many entries", nil)
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
			return faults.Invalid("metadata bundle archive contains invalid path traversal entry", nil)
		}

		targetPath := filepath.Join(destination, entryPath)
		if !fsutil.IsPathUnderRoot(destination, targetPath) {
			return faults.Invalid("metadata bundle archive contains path outside extraction root", nil)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return faults.Internal("failed to create bundle extraction directory", err)
			}
		case tar.TypeReg:
			if header.Size < 0 || header.Size > maxArchiveFileSizeByte {
				return faults.Invalid("metadata bundle archive contains oversized file entry", nil)
			}
			totalBytes += header.Size
			if totalBytes > maxTotalArchiveBytes {
				return faults.Invalid("metadata bundle archive exceeds maximum total size", nil)
			}
			if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
				return faults.Internal("failed to create bundle extraction parent directory", err)
			}

			output, err := os.OpenFile(targetPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
			if err != nil {
				return faults.Internal("failed to create bundle extraction file", err)
			}

			limitedReader := io.LimitReader(tarReader, maxArchiveFileSizeByte+1)
			written, copyErr := io.Copy(output, limitedReader)
			closeErr := output.Close()
			if copyErr != nil {
				return faults.Internal("failed to extract bundle archive file", copyErr)
			}
			if closeErr != nil {
				return faults.Internal("failed to finalize bundle archive file", closeErr)
			}
			if written > maxArchiveFileSizeByte {
				return faults.Invalid("metadata bundle archive contains oversized file entry", nil)
			}
		default:
			return faults.Invalid("metadata bundle archive contains unsupported entry type", nil)
		}
	}
}
