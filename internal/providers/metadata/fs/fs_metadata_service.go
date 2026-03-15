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

package fsmetadata

import (
	"path/filepath"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
)

var _ metadatadomain.MetadataService = (*FSMetadataService)(nil)
var _ metadatadomain.ResourceOperationSpecRenderer = (*FSMetadataService)(nil)
var _ metadatadomain.DefaultsArtifactStore = (*FSMetadataService)(nil)

type metadataPathKind int

const (
	metadataPathResource metadataPathKind = iota
	metadataPathCollection
)

type FSMetadataService struct {
	baseDir string
}

func NewFSMetadataService(baseDir string) *FSMetadataService {
	return &FSMetadataService{
		baseDir: filepath.Clean(baseDir),
	}
}

func notFoundError(message string) error {
	return faults.NewTypedError(faults.NotFoundError, message, nil)
}

func internalError(message string, cause error) error {
	return faults.NewTypedError(faults.InternalError, message, cause)
}

func metadataPathKindName(kind metadataPathKind) string {
	switch kind {
	case metadataPathCollection:
		return "collection"
	case metadataPathResource:
		return "resource"
	default:
		return "unknown"
	}
}
