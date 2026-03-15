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

package orchestrator

import (
	"context"
	"path"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
)

func (r *Orchestrator) requireRepository() (repository.ResourceStore, error) {
	if r == nil || r.repository == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "repository store is not configured", nil)
	}
	return r.repository, nil
}

func (r *Orchestrator) requireServer() (managedserver.ManagedServerClient, error) {
	if r == nil || r.server == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "managed server is not configured", nil)
	}
	return r.server, nil
}

func (r *Orchestrator) resolveMetadataForPath(
	ctx context.Context,
	normalizedPath string,
	allowMissing bool,
) (metadata.ResourceMetadata, error) {
	if r == nil || r.metadata == nil {
		if allowMissing {
			return metadata.ResourceMetadata{}, nil
		}
		return metadata.ResourceMetadata{}, faults.NewTypedError(faults.ValidationError, "metadata service is not configured", nil)
	}

	resolvedMetadata, err := r.metadata.ResolveForPath(ctx, normalizedPath)
	if err != nil {
		if allowMissing && faults.IsCategory(err, faults.NotFoundError) {
			debugctx.Printf(ctx, "metadata missing for path=%q; using empty metadata fallback", normalizedPath)
			return metadata.ResourceMetadata{}, nil
		}
		return metadata.ResourceMetadata{}, err
	}

	return resolvedMetadata, nil
}

func collectionPathFor(normalizedPath string) string {
	if normalizedPath == "/" {
		return "/"
	}

	collectionPath := path.Dir(normalizedPath)
	if collectionPath == "." || collectionPath == "" {
		return "/"
	}

	return collectionPath
}
