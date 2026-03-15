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
	"context"
	"fmt"
	"sort"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func (s *FSMetadataService) resolveMetadataDefaults(
	ctx context.Context,
	selector string,
	kind metadataPathKind,
	item metadatadomain.ResourceMetadata,
) (metadatadomain.ResourceMetadata, error) {
	if item.Defaults == nil {
		return item, nil
	}

	resolved := metadatadomain.CloneResourceMetadata(item)
	resolved.Defaults = metadatadomain.CloneDefaultsSpec(item.Defaults)

	value, err := s.resolveDefaultsEntry(ctx, selector, kind, "resource.defaults.value", resolved.Defaults.Value)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	resolved.Defaults.Value = value

	if resolved.Defaults.Profiles == nil {
		return resolved, nil
	}

	keys := make([]string, 0, len(resolved.Defaults.Profiles))
	for key := range resolved.Defaults.Profiles {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value, err := s.resolveDefaultsEntry(
			ctx,
			selector,
			kind,
			"resource.defaults.profiles."+key,
			resolved.Defaults.Profiles[key],
		)
		if err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}
		resolved.Defaults.Profiles[key] = value
	}

	return resolved, nil
}

func (s *FSMetadataService) resolveDefaultsEntry(
	ctx context.Context,
	selector string,
	kind metadataPathKind,
	field string,
	value any,
) (any, error) {
	if value == nil {
		return nil, nil
	}

	includeFile, ok := value.(string)
	if !ok {
		normalized, err := resource.Normalize(value)
		if err != nil {
			return nil, err
		}
		objectValue, ok := normalized.(map[string]any)
		if !ok {
			return nil, faults.NewValidationError(field+" must resolve to an object", nil)
		}
		return objectValue, nil
	}

	resolvedFile, ok := metadatadomain.ParseDefaultsIncludeReference(includeFile)
	if !ok {
		return nil, faults.NewValidationError(field+" must be an exact {{include ...}} reference", nil)
	}

	metadataPath := metadataPathForSelector(selector, kind)
	content, err := s.ReadDefaultsArtifact(ctx, metadataPath, resolvedFile)
	if err != nil {
		return nil, faults.NewValidationError(
			fmt.Sprintf("%s failed to resolve include %q", field, includeFile),
			err,
		)
	}
	return content.Value, nil
}

func metadataPathForSelector(selector string, kind metadataPathKind) string {
	if kind == metadataPathCollection {
		if selector == "/" {
			return "/_"
		}
		return selector + "/_"
	}
	return selector
}
