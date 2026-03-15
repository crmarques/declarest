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

package pathfallback

import (
	"context"
	"path"
	"strings"

	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

// ShouldUseMetadataCollectionFallback returns true when metadata declares the
// requested path segment as a literal child collection branch under its parent.
// Wildcard item selectors (for example "/projects/_") do not qualify because
// they identify resource items, not nested collection branches.
func ShouldUseMetadataCollectionFallback(
	ctx context.Context,
	metadataService metadatadomain.MetadataService,
	logicalPath string,
	_ []resource.Resource,
) bool {
	collectionChildrenResolver, ok := metadataService.(metadatadomain.CollectionChildrenResolver)
	if !ok {
		return false
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil || normalizedPath == "/" {
		return false
	}

	parentPath := path.Dir(normalizedPath)
	if parentPath == "." || parentPath == "" {
		parentPath = "/"
	}
	requestedSegment := path.Base(normalizedPath)
	if strings.TrimSpace(requestedSegment) == "" || requestedSegment == "/" {
		return false
	}

	children, err := collectionChildrenResolver.ResolveCollectionChildren(ctx, parentPath)
	if err != nil {
		return false
	}
	for _, child := range children {
		if strings.TrimSpace(child) == requestedSegment {
			return true
		}
	}

	return false
}
