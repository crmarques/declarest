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

package defaults

import (
	"context"

	appdeps "github.com/crmarques/declarest/internal/app/deps"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func GetLocalProfile(ctx context.Context, deps Dependencies, logicalPath string, profile string) (Result, error) {
	target, err := resolveScopeTarget(ctx, deps, logicalPath)
	if err != nil {
		return Result{}, err
	}
	if err := metadata.ValidateDefaultsProfileName(profile); err != nil {
		return Result{}, err
	}

	rawMetadata, err := getRawMetadataForPath(ctx, deps, target.metadataPath)
	if err != nil {
		return Result{}, err
	}

	var profileValue any
	if rawMetadata.Defaults != nil && rawMetadata.Defaults.Profiles != nil {
		profileValue = rawMetadata.Defaults.Profiles[profile]
	}
	content, err := localDefaultsEntryContent(ctx, deps, target.metadataPath, target.payloadDescriptor, profile, profileValue)
	if err != nil {
		return Result{}, err
	}
	return Result{
		ResolvedPath: target.scopePath,
		Content:      content,
	}, nil
}

func GetProfile(ctx context.Context, deps Dependencies, logicalPath string, profile string) (Result, error) {
	target, err := resolveScopeTarget(ctx, deps, logicalPath)
	if err != nil {
		return Result{}, err
	}
	if err := metadata.ValidateDefaultsProfileName(profile); err != nil {
		return Result{}, err
	}

	metadataService, err := appdeps.RequireMetadataService(deps)
	if err != nil {
		return Result{}, err
	}
	resolvedMetadata, err := metadataService.ResolveForPath(ctx, target.metadataPath)
	if err != nil {
		return Result{}, err
	}

	content := resource.Content{
		Value:      map[string]any{},
		Descriptor: target.payloadDescriptor,
	}
	if metadata.HasDefaultsSpecDirectives(resolvedMetadata.Defaults) {
		if entry, ok := resolvedMetadata.Defaults.Profiles[profile]; ok {
			content.Value = normalizeEmptyDefaultsValue(entry)
		}
	}
	return Result{
		ResolvedPath: target.scopePath,
		Content:      content,
	}, nil
}

func SaveProfile(ctx context.Context, deps Dependencies, logicalPath string, profile string, content resource.Content) (Result, error) {
	target, err := resolveScopeTarget(ctx, deps, logicalPath)
	if err != nil {
		return Result{}, err
	}
	if err := metadata.ValidateDefaultsProfileName(profile); err != nil {
		return Result{}, err
	}

	rawMetadata, err := getRawMetadataForPath(ctx, deps, target.metadataPath)
	if err != nil {
		return Result{}, err
	}
	if rawMetadata.Defaults == nil {
		rawMetadata.Defaults = &metadata.DefaultsSpec{}
	}
	if rawMetadata.Defaults.Profiles == nil {
		rawMetadata.Defaults.Profiles = map[string]any{}
	}

	fileName, descriptor, err := resolveManagedDefaultsFile(profile, rawMetadata.Defaults.Profiles[profile], target.payloadDescriptor)
	if err != nil {
		return Result{}, err
	}
	cleared, err := saveManagedDefaultsEntry(ctx, deps, target.metadataPath, fileName, descriptor, content)
	if err != nil {
		return Result{}, err
	}
	if cleared {
		delete(rawMetadata.Defaults.Profiles, profile)
	} else {
		rawMetadata.Defaults.Profiles[profile] = metadata.DefaultsIncludePlaceholder(fileName)
	}
	if len(rawMetadata.Defaults.Profiles) == 0 {
		rawMetadata.Defaults.Profiles = nil
	}
	if !metadata.HasDefaultsSpecDirectives(rawMetadata.Defaults) {
		rawMetadata.Defaults = nil
	}
	if err := writeRawMetadataForPath(ctx, deps, target.metadataPath, rawMetadata); err != nil {
		return Result{}, err
	}

	return Result{
		ResolvedPath: target.scopePath,
		Content: resource.Content{
			Value:      normalizeEmptyDefaultsValue(content.Value),
			Descriptor: descriptor,
		},
	}, nil
}

func DeleteProfile(ctx context.Context, deps Dependencies, logicalPath string, profile string) error {
	target, err := resolveScopeTarget(ctx, deps, logicalPath)
	if err != nil {
		return err
	}
	if err := metadata.ValidateDefaultsProfileName(profile); err != nil {
		return err
	}

	rawMetadata, err := getRawMetadataForPath(ctx, deps, target.metadataPath)
	if err != nil {
		return err
	}
	if rawMetadata.Defaults == nil || rawMetadata.Defaults.Profiles == nil {
		return nil
	}

	entry, found := rawMetadata.Defaults.Profiles[profile]
	if !found {
		return nil
	}
	if managedFile, ok := managedDefaultsFile(entry, profile); ok {
		if artifactStore, err := requireDefaultsArtifactStore(deps); err == nil {
			if deleteErr := artifactStore.DeleteDefaultsArtifact(ctx, target.metadataPath, managedFile); deleteErr != nil {
				return deleteErr
			}
		}
	}

	delete(rawMetadata.Defaults.Profiles, profile)
	if len(rawMetadata.Defaults.Profiles) == 0 {
		rawMetadata.Defaults.Profiles = nil
	}
	if !metadata.HasDefaultsSpecDirectives(rawMetadata.Defaults) {
		rawMetadata.Defaults = nil
	}
	return writeRawMetadataForPath(ctx, deps, target.metadataPath, rawMetadata)
}
