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
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	"github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
)

type ConfigResult struct {
	ResolvedPath string
	Defaults     metadata.DefaultsSpec
}

type scopeTarget struct {
	scopePath         string
	metadataPath      string
	concretePath      string
	resourceContent   resource.Content
	payloadDescriptor resource.PayloadDescriptor
}

func GetLocalBaseline(ctx context.Context, deps Dependencies, logicalPath string) (Result, error) {
	target, err := resolveScopeTarget(ctx, deps, logicalPath)
	if err != nil {
		return Result{}, err
	}

	rawMetadata, err := getRawMetadataForPath(ctx, deps, target.metadataPath)
	if err != nil {
		return Result{}, err
	}

	content, err := localDefaultsEntryContent(ctx, deps, target.metadataPath, target.payloadDescriptor, "", nilValue(rawMetadata.Defaults, func(value *metadata.DefaultsSpec) any {
		return value.Value
	}))
	if err != nil {
		return Result{}, err
	}
	return Result{
		ResolvedPath: target.scopePath,
		Content:      content,
	}, nil
}

func GetConfig(ctx context.Context, deps Dependencies, logicalPath string) (ConfigResult, error) {
	target, err := resolveScopeTarget(ctx, deps, logicalPath)
	if err != nil {
		return ConfigResult{}, err
	}

	rawMetadata, err := getRawMetadataForPath(ctx, deps, target.metadataPath)
	if err != nil {
		return ConfigResult{}, err
	}

	defaultsValue := metadata.DefaultsSpec{}
	if rawMetadata.Defaults != nil {
		defaultsValue = *metadata.CloneDefaultsSpec(rawMetadata.Defaults)
	}
	return ConfigResult{
		ResolvedPath: target.scopePath,
		Defaults:     defaultsValue,
	}, nil
}

func SaveConfig(ctx context.Context, deps Dependencies, logicalPath string, value metadata.DefaultsSpec) (ConfigResult, error) {
	target, err := resolveScopeTarget(ctx, deps, logicalPath)
	if err != nil {
		return ConfigResult{}, err
	}

	rawMetadata, err := getRawMetadataForPath(ctx, deps, target.metadataPath)
	if err != nil {
		return ConfigResult{}, err
	}
	previousDefaults := metadata.CloneDefaultsSpec(rawMetadata.Defaults)

	if !metadata.HasDefaultsSpecDirectives(&value) {
		rawMetadata.Defaults = nil
	} else {
		rawMetadata.Defaults = metadata.CloneDefaultsSpec(&value)
	}
	if err := cleanupManagedDefaultsArtifacts(ctx, deps, target.metadataPath, previousDefaults, rawMetadata.Defaults); err != nil {
		return ConfigResult{}, err
	}
	if err := writeRawMetadataForPath(ctx, deps, target.metadataPath, rawMetadata); err != nil {
		return ConfigResult{}, err
	}

	current := metadata.DefaultsSpec{}
	if rawMetadata.Defaults != nil {
		current = *metadata.CloneDefaultsSpec(rawMetadata.Defaults)
	}
	return ConfigResult{
		ResolvedPath: target.scopePath,
		Defaults:     current,
	}, nil
}

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

func saveBaseline(ctx context.Context, deps Dependencies, logicalPath string, content resource.Content) (Result, error) {
	target, err := resolveScopeTarget(ctx, deps, logicalPath)
	if err != nil {
		return Result{}, err
	}

	rawMetadata, err := getRawMetadataForPath(ctx, deps, target.metadataPath)
	if err != nil {
		return Result{}, err
	}
	if rawMetadata.Defaults == nil {
		rawMetadata.Defaults = &metadata.DefaultsSpec{}
	}

	fileName, descriptor, err := resolveManagedDefaultsFile("", rawMetadata.Defaults.Value, target.payloadDescriptor)
	if err != nil {
		return Result{}, err
	}
	cleared, err := saveManagedDefaultsEntry(ctx, deps, target.metadataPath, fileName, descriptor, content)
	if err != nil {
		return Result{}, err
	}
	if cleared {
		rawMetadata.Defaults.Value = nil
	} else {
		rawMetadata.Defaults.Value = metadata.DefaultsIncludePlaceholder(fileName)
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

func resolveScopeTarget(ctx context.Context, deps Dependencies, logicalPath string) (scopeTarget, error) {
	parsedPath, err := resource.ParseRawPathWithOptions(logicalPath, resource.RawPathParseOptions{})
	if err != nil {
		return scopeTarget{}, err
	}
	if parsedPath.Normalized == "/" {
		return scopeTarget{}, faults.NewValidationError("logical path must target a resource or collection, not root", nil)
	}

	pathDescriptor, err := metadata.ParsePathDescriptor(logicalPath)
	if err != nil {
		return scopeTarget{}, err
	}

	collectionTarget := parsedPath.ExplicitCollectionTarget || pathDescriptor.Collection
	if !collectionTarget {
		orchestratorService, orchestratorErr := appdeps.RequireOrchestrator(deps)
		if orchestratorErr != nil {
			return scopeTarget{}, orchestratorErr
		}
		resourceTarget, resourceErr := resolveResolvedLocalTarget(ctx, orchestratorService, parsedPath.Normalized)
		if resourceErr == nil {
			descriptor, descriptorErr := resolveTargetPayloadDescriptor(ctx, deps, parsedPath.Normalized, resourceTarget.PayloadDescriptor)
			if descriptorErr != nil {
				return scopeTarget{}, descriptorErr
			}
			return scopeTarget{
				scopePath:    parsedPath.Normalized,
				metadataPath: parsedPath.Normalized,
				concretePath: parsedPath.Normalized,
				resourceContent: resource.Content{
					Value:      resourceTarget.Payload,
					Descriptor: resourceTarget.PayloadDescriptor,
				},
				payloadDescriptor: descriptor,
			}, nil
		}
		if !faults.IsCategory(resourceErr, faults.NotFoundError) {
			return scopeTarget{}, resourceErr
		}
		collectionTarget = true
	}

	scopePath := pathDescriptor.Selector
	metadataPath := collectionMetadataPath(scopePath)
	concretePath, resourceContent, _ := resolveFirstCollectionResource(ctx, deps, scopePath)
	descriptor, err := resolveTargetPayloadDescriptor(ctx, deps, metadataPath, resourceContent.Descriptor)
	if err != nil {
		return scopeTarget{}, err
	}
	return scopeTarget{
		scopePath:         scopePath,
		metadataPath:      metadataPath,
		concretePath:      concretePath,
		resourceContent:   resourceContent,
		payloadDescriptor: descriptor,
	}, nil
}

func resolveEffectiveDefaultsForPath(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	fallback resource.PayloadDescriptor,
) (resource.Content, bool, error) {
	metadataService, err := appdeps.RequireMetadataService(deps)
	if err != nil {
		return resource.Content{}, false, err
	}
	resolvedMetadata, err := metadataService.ResolveForPath(ctx, logicalPath)
	if err != nil {
		return resource.Content{}, false, err
	}
	if !metadata.HasDefaultsSpecDirectives(resolvedMetadata.Defaults) {
		return resource.Content{}, false, nil
	}
	value, err := metadata.ResolveEffectiveDefaults(resolvedMetadata.Defaults)
	if err != nil {
		return resource.Content{}, false, err
	}
	descriptor, err := resolveTargetPayloadDescriptor(ctx, deps, logicalPath, fallback)
	if err != nil {
		return resource.Content{}, false, err
	}
	return resource.Content{
		Value:      value,
		Descriptor: descriptor,
	}, true, nil
}

func resolveTargetPayloadDescriptor(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	fallback resource.PayloadDescriptor,
) (resource.PayloadDescriptor, error) {
	if resource.IsPayloadDescriptorExplicit(fallback) {
		return resource.NormalizePayloadDescriptor(fallback), nil
	}

	metadataService := deps.MetadataService()
	if metadataService != nil {
		md, err := metadataService.ResolveForPath(ctx, logicalPath)
		if err == nil {
			payloadType, payloadTypeErr := metadata.EffectivePayloadType(md, resource.PayloadTypeJSON)
			if payloadTypeErr == nil {
				return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: payloadType}), nil
			}
		} else if !faults.IsCategory(err, faults.NotFoundError) {
			return resource.PayloadDescriptor{}, err
		}
	}

	return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}), nil
}

func resolveFirstCollectionResource(
	ctx context.Context,
	deps Dependencies,
	collectionPath string,
) (string, resource.Content, error) {
	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return "", resource.Content{}, err
	}
	items, err := orchestratorService.ListLocal(ctx, collectionPath, orchestratordomain.ListPolicy{})
	if err != nil {
		return "", resource.Content{}, err
	}

	directChildren := make([]string, 0, len(items))
	for _, item := range items {
		candidatePath := strings.TrimSpace(item.LogicalPath)
		if candidatePath == "" {
			continue
		}
		if _, ok := resource.ChildSegment(collectionPath, candidatePath); !ok {
			continue
		}
		directChildren = append(directChildren, candidatePath)
	}
	if len(directChildren) == 0 {
		return "", resource.Content{}, faults.NewTypedError(
			faults.NotFoundError,
			fmt.Sprintf("resource %q not found", collectionPath),
			nil,
		)
	}

	sort.Strings(directChildren)
	content, err := orchestratorService.GetLocal(ctx, directChildren[0])
	if err != nil {
		return "", resource.Content{}, err
	}
	return directChildren[0], content, nil
}

func collectionMetadataPath(collectionPath string) string {
	if strings.TrimSpace(collectionPath) == "/" {
		return "/_"
	}
	return path.Clean(collectionPath) + "/_"
}

func getRawMetadataForPath(ctx context.Context, deps Dependencies, logicalPath string) (metadata.ResourceMetadata, error) {
	metadataService, err := appdeps.RequireMetadataService(deps)
	if err != nil {
		return metadata.ResourceMetadata{}, err
	}
	value, err := metadataService.Get(ctx, logicalPath)
	if err == nil {
		return value, nil
	}
	if faults.IsCategory(err, faults.NotFoundError) {
		return metadata.ResourceMetadata{}, nil
	}
	return metadata.ResourceMetadata{}, err
}

func writeRawMetadataForPath(ctx context.Context, deps Dependencies, logicalPath string, value metadata.ResourceMetadata) error {
	metadataService, err := appdeps.RequireMetadataService(deps)
	if err != nil {
		return err
	}
	if metadata.HasResourceMetadataDirectives(value) {
		return metadataService.Set(ctx, logicalPath, value)
	}
	return metadataService.Unset(ctx, logicalPath)
}

func requireDefaultsArtifactStore(deps Dependencies) (metadata.DefaultsArtifactStore, error) {
	metadataService, err := appdeps.RequireMetadataService(deps)
	if err != nil {
		return nil, err
	}
	artifactStore, ok := metadataService.(metadata.DefaultsArtifactStore)
	if !ok {
		return nil, faults.NewValidationError("resource defaults artifacts are not supported by the configured metadata service", nil)
	}
	return artifactStore, nil
}

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
	if objectValue == nil || len(objectValue) == 0 {
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

func nilValue[T any](value *T, selectFn func(*T) any) any {
	if value == nil {
		return nil
	}
	return selectFn(value)
}
