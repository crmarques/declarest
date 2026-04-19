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
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/resource"
)

func resolveManagedDefaultsFile(profile string, currentEntry any, fallback resource.PayloadDescriptor) (string, resource.PayloadDescriptor, error) {
	if managedFile, ok := managedDefaultsFile(currentEntry, profile); ok {
		descriptor, ok := resource.PayloadDescriptorForFileName(managedFile)
		if ok {
			return managedFile, resource.NormalizePayloadDescriptor(descriptor), nil
		}
	}

	descriptor := preferredManagedDefaultsDescriptor(fallback)
	fileName, err := metadata.DefaultsFileName(profile, descriptor)
	if err != nil {
		return "", resource.PayloadDescriptor{}, err
	}
	return fileName, descriptor, nil
}

func preferredManagedDefaultsDescriptor(fallback resource.PayloadDescriptor) resource.PayloadDescriptor {
	resolved := resource.NormalizePayloadDescriptor(fallback)
	switch resolved.PayloadType {
	case resource.PayloadTypeJSON:
		return resolved
	case resource.PayloadTypeProperties:
		return resolved
	case resource.PayloadTypeYAML:
		return resolved
	default:
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeYAML})
	}
}

func saveManagedDefaultsEntry(
	ctx context.Context,
	deps Dependencies,
	metadataPath string,
	fileName string,
	descriptor resource.PayloadDescriptor,
	content resource.Content,
) (bool, error) {
	normalizedValue := normalizeEmptyDefaultsValue(content.Value)
	objectValue, _ := normalizedValue.(map[string]any)
	if len(objectValue) == 0 {
		if managedStore, err := requireDefaultsArtifactStore(deps); err == nil {
			_ = managedStore.DeleteDefaultsArtifact(ctx, metadataPath, fileName)
		}
		return true, nil
	}

	artifactStore, err := requireDefaultsArtifactStore(deps)
	if err != nil {
		return false, err
	}
	if err := artifactStore.WriteDefaultsArtifact(ctx, metadataPath, fileName, resource.Content{
		Value:      objectValue,
		Descriptor: descriptor,
	}); err != nil {
		return false, err
	}
	return false, nil
}

func cleanupManagedDefaultsArtifacts(
	ctx context.Context,
	deps Dependencies,
	metadataPath string,
	previous *metadata.DefaultsSpec,
	next *metadata.DefaultsSpec,
) error {
	artifactStore, err := requireDefaultsArtifactStore(deps)
	if err != nil {
		return err
	}

	if previous != nil {
		if file, ok := managedDefaultsFile(previous.Value, ""); ok && (next == nil || !referencesManagedDefaultsFile(next.Value, "", file)) {
			if err := artifactStore.DeleteDefaultsArtifact(ctx, metadataPath, file); err != nil {
				return err
			}
		}
	}

	previousProfiles := map[string]any{}
	if previous != nil && previous.Profiles != nil {
		previousProfiles = previous.Profiles
	}
	nextProfiles := map[string]any{}
	if next != nil && next.Profiles != nil {
		nextProfiles = next.Profiles
	}
	for key, entry := range previousProfiles {
		file, ok := managedDefaultsFile(entry, key)
		if !ok {
			continue
		}
		if referencesManagedDefaultsFile(nextProfiles[key], key, file) {
			continue
		}
		if err := artifactStore.DeleteDefaultsArtifact(ctx, metadataPath, file); err != nil {
			return err
		}
	}
	return nil
}

func managedDefaultsFile(value any, profile string) (string, bool) {
	includeRef, ok := value.(string)
	if !ok {
		return "", false
	}
	file, ok := metadata.ParseDefaultsIncludeReference(includeRef)
	if !ok {
		return "", false
	}
	var validationErr error
	if strings.TrimSpace(profile) == "" {
		validationErr = metadata.ValidateDefaultsSpec(&metadata.DefaultsSpec{Value: includeRef})
	} else {
		validationErr = metadata.ValidateDefaultsSpec(&metadata.DefaultsSpec{
			Profiles: map[string]any{profile: includeRef},
		})
	}
	return file, validationErr == nil
}

func referencesManagedDefaultsFile(value any, profile string, expectedFile string) bool {
	file, ok := managedDefaultsFile(value, profile)
	return ok && file == expectedFile
}

func localDefaultsEntryContent(
	ctx context.Context,
	deps Dependencies,
	metadataPath string,
	fallback resource.PayloadDescriptor,
	profile string,
	value any,
) (resource.Content, error) {
	descriptor := preferredManagedDefaultsDescriptor(fallback)
	if managedFile, ok := managedDefaultsFile(value, profile); ok {
		artifactStore, err := requireDefaultsArtifactStore(deps)
		if err != nil {
			return resource.Content{}, err
		}
		content, err := artifactStore.ReadDefaultsArtifact(ctx, metadataPath, managedFile)
		if err != nil {
			return resource.Content{}, err
		}
		return resource.Content{
			Value:      normalizeEmptyDefaultsValue(content.Value),
			Descriptor: content.Descriptor,
		}, nil
	}

	normalized := normalizeEmptyDefaultsValue(value)
	return resource.Content{
		Value:      normalized,
		Descriptor: descriptor,
	}, nil
}
