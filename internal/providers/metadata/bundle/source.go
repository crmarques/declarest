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
)

const (
	sourceKindLocal = "local"
	sourceKindURL   = "url"
	sourceKindShort = "shorthand"
)

var shorthandNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

var versionedArtifactStemPattern = regexp.MustCompile(
	`^([a-z0-9][a-z0-9-]*)-(v?[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?)$`,
)

var shorthandReleaseBaseURL = "https://github.com"

type bundleSource struct {
	kind             string
	cacheDirName     string
	localPath        string
	remoteURL        string
	shorthandName    string
	shorthandVersion string
}

func parseBundleSource(ref string) (bundleSource, error) {
	value := strings.TrimSpace(ref)
	if value == "" {
		return bundleSource{}, faults.Invalid("metadata.bundle is empty", nil)
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
			cacheDirName := cacheDirNameForSourceArtifact(
				filepath.Base(parsed.Path),
				"url:"+parsed.String(),
			)
			return bundleSource{
				kind:         sourceKindURL,
				remoteURL:    parsed.String(),
				cacheDirName: cacheDirName,
			}, nil
		default:
			return bundleSource{}, faults.Invalid("metadata.bundle URL must use http or https", nil)
		}
	}

	absolutePath, err := filepath.Abs(value)
	if err != nil {
		return bundleSource{}, faults.Invalid("metadata.bundle local path is invalid", err)
	}

	cacheDirName := cacheDirNameForSourceArtifact(
		filepath.Base(absolutePath),
		"local:"+absolutePath,
	)
	return bundleSource{
		kind:         sourceKindLocal,
		localPath:    absolutePath,
		cacheDirName: cacheDirName,
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

func parseVersionedArtifactFileName(fileName string) (string, string, bool) {
	value := strings.TrimSpace(fileName)
	if value == "" {
		return "", "", false
	}

	lowerValue := strings.ToLower(value)
	if !strings.HasSuffix(lowerValue, ".tar.gz") {
		return "", "", false
	}
	stem := value[:len(value)-len(".tar.gz")]
	matches := versionedArtifactStemPattern.FindStringSubmatch(stem)
	if len(matches) != 3 {
		return "", "", false
	}

	version, err := normalizeSemver(matches[2])
	if err != nil {
		return "", "", false
	}
	return matches[1], version, true
}
