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

package fsstore

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/resource"
)

type payloadFileInfo struct {
	Path       string
	Name       string
	Descriptor resource.PayloadDescriptor
	Shared     bool
}

type resourcePayloadFiles struct {
	Resource *payloadFileInfo
	Defaults *payloadFileInfo
}

func (f resourcePayloadFiles) primary() *payloadFileInfo {
	return f.Resource
}

func firstMetadataBaseDir(values []string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		return filepath.Clean(trimmed)
	}
	return ""
}

func (r *LocalResourceRepository) discoverPayloadFiles(logicalPath string) (resourcePayloadFiles, error) {
	resourceDir, err := r.collectionDirPath(logicalPath)
	if err != nil {
		return resourcePayloadFiles{}, err
	}
	return r.payloadFilesInfoFromDir(logicalPath, resourceDir)
}

func (r *LocalResourceRepository) payloadFilesInfoFromDir(logicalPath string, dirPath string) (resourcePayloadFiles, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return resourcePayloadFiles{}, nil
		}
		return resourcePayloadFiles{}, faults.Internal("failed to inspect resource directory", err)
	}

	resourceCandidates := make([]string, 0, 1)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		switch {
		case strings.HasPrefix(entry.Name(), "resource."):
			resourceCandidates = append(resourceCandidates, entry.Name())
		}
	}

	resourceInfo, err := payloadFileInfoFromCandidates(logicalPath, dirPath, "resource", resourceCandidates)
	if err != nil {
		return resourcePayloadFiles{}, err
	}

	files := resourcePayloadFiles{
		Resource: resourceInfo,
	}
	return files, nil
}

func (r *LocalResourceRepository) resolvePayloadTarget(
	logicalPath string,
	content resource.Content,
) (payloadFileInfo, resourcePayloadFiles, error) {
	files, err := r.discoverPayloadFiles(logicalPath)
	if err != nil {
		return payloadFileInfo{}, resourcePayloadFiles{}, err
	}

	desired := desiredPayloadDescriptor(content, files)
	canonicalPath, err := r.canonicalPayloadFilePath(logicalPath, desired.Extension)
	if err != nil {
		return payloadFileInfo{}, resourcePayloadFiles{}, err
	}

	target := payloadFileInfo{
		Path:       canonicalPath,
		Name:       "resource" + desired.Extension,
		Descriptor: desired,
	}
	return target, files, nil
}

func desiredPayloadDescriptor(content resource.Content, existing resourcePayloadFiles) resource.PayloadDescriptor {
	if resource.IsPayloadDescriptorExplicit(content.Descriptor) {
		return resource.NormalizePayloadDescriptor(content.Descriptor)
	}
	if existing.Resource != nil {
		return existing.Resource.Descriptor
	}
	if resource.IsBinaryValue(content.Value) {
		return resource.DefaultOctetStreamDescriptor()
	}
	return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON})
}

func payloadFileInfoFromCandidates(
	logicalPath string,
	dirPath string,
	baseName string,
	candidates []string,
) (*payloadFileInfo, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	sort.Strings(candidates)
	if len(candidates) > 1 {
		label := "payload"
		if baseName == "defaults" {
			label = "defaults"
		}
		return nil, faults.Conflict(
			fmt.Sprintf("resource %q has multiple %s files: %s", logicalPath, label, strings.Join(candidates, ", ")),
			nil,
		)
	}

	name := candidates[0]
	return &payloadFileInfo{
		Path: filepath.Join(dirPath, name),
		Name: name,
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{
			Extension: filepath.Ext(name),
		}),
	}, nil
}
