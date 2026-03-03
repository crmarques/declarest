package controllers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxArtifactDownloadSize = 256 << 20 // 256 MB

var sharedHTTPClient = &http.Client{
	Timeout: 60 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        20,
		MaxIdleConnsPerHost: 5,
		IdleConnTimeout:     90 * time.Second,
	},
}

func downloadArtifact(ctx context.Context, artifactURL string, destDir string) (string, error) {
	trimmedURL := strings.TrimSpace(artifactURL)
	if trimmedURL == "" {
		return "", nil
	}
	if err := ensureDir(destDir); err != nil {
		return "", err
	}

	hash := sha256.Sum256([]byte(trimmedURL))
	fileName := hex.EncodeToString(hash[:])
	parsed, err := url.Parse(trimmedURL)
	if err == nil {
		ext := filepath.Ext(parsed.Path)
		if ext != "" {
			fileName = fileName + ext
		}
	}
	targetPath := filepath.Join(destDir, fileName)
	tmpPath := targetPath + ".tmp"

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, trimmedURL, nil)
	if err != nil {
		return "", fmt.Errorf("build download request: %w", err)
	}

	// If a cached file already exists, send If-Modified-Since so the server
	// can respond with 304 Not Modified and avoid a redundant download.
	if info, statErr := os.Stat(targetPath); statErr == nil {
		request.Header.Set("If-Modified-Since", info.ModTime().UTC().Format(http.TimeFormat))
	}

	response, err := sharedHTTPClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("download artifact %s: %w", sanitizeURL(trimmedURL), err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusNotModified {
		return targetPath, nil
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("download artifact %s: unexpected status code %d", sanitizeURL(trimmedURL), response.StatusCode)
	}

	limitedBody := io.LimitReader(response.Body, maxArtifactDownloadSize+1)

	outputFile, err := os.OpenFile(tmpPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return "", fmt.Errorf("open artifact cache file: %w", err)
	}
	written, copyErr := io.Copy(outputFile, limitedBody)
	if copyErr != nil {
		_ = outputFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("write artifact cache file: %w", copyErr)
	}
	if written > maxArtifactDownloadSize {
		_ = outputFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("artifact %s exceeds maximum download size of %d bytes", sanitizeURL(trimmedURL), maxArtifactDownloadSize)
	}
	if err := outputFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("finalize artifact cache file: %w", err)
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("promote artifact cache file: %w", err)
	}
	return targetPath, nil
}
