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

package save

import (
	"context"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	defaultsapp "github.com/crmarques/declarest/internal/app/resource/defaults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

type Dependencies = appdeps.Dependencies

type ExecuteOptions struct {
	AsItems        bool
	AsOneResource  bool
	Secret         bool
	AllowPlaintext bool
	Force          bool
	PruneDefaults  bool

	SecretAttributesEnabled   bool
	RequestedSecretAttributes []string
	SkipItems                 []string
}

func Execute(
	ctx context.Context,
	deps Dependencies,
	resolvedPath string,
	value resource.Content,
	hasInput bool,
	options ExecuteOptions,
) error {
	normalizedPath, hasWildcard, explicitCollectionTarget, err := normalizeSavePathPattern(resolvedPath)
	if err != nil {
		return err
	}
	if options.AsItems && options.AsOneResource {
		return faults.NewValidationError("flag --mode must choose a single save mode", nil)
	}
	if options.Secret && options.AsItems {
		return faults.NewValidationError("flags --secret and --mode items cannot be used together", nil)
	}
	if options.AsOneResource && len(options.SkipItems) > 0 {
		return faults.NewValidationError("flag --exclude is not supported with --mode single", nil)
	}
	if options.Secret && len(options.SkipItems) > 0 {
		return faults.NewValidationError("flag --exclude is not supported with --secret", nil)
	}
	if options.Secret && options.SecretAttributesEnabled {
		return faults.NewValidationError("flags --secret and --secret-attributes cannot be used together", nil)
	}
	if options.Secret && options.AllowPlaintext {
		return faults.NewValidationError("flags --secret and --allow-plaintext cannot be used together", nil)
	}

	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return err
	}
	repositoryService, err := appdeps.RequireResourceStore(deps)
	if err != nil {
		return err
	}

	if hasWildcard {
		if hasInput {
			return faults.NewValidationError("wildcard save paths are supported only when reading from remote server", nil)
		}

		targets, err := expandSaveWildcardPaths(ctx, orchestratorService, normalizedPath)
		if err != nil {
			return err
		}

		matchedCount := 0
		for _, targetPath := range targets {
			remoteValue, err := orchestratorService.GetRemote(ctx, targetPath)
			if err != nil {
				if faults.IsCategory(err, faults.NotFoundError) {
					continue
				}
				return err
			}
			matchedCount++

			if err := saveResolvedPathPayload(
				ctx,
				deps,
				orchestratorService,
				repositoryService,
				targetPath,
				remoteValue,
				options.AsItems,
				options.AsOneResource,
				options.Secret,
				options.AllowPlaintext,
				options.PruneDefaults,
				options.SecretAttributesEnabled,
				options.RequestedSecretAttributes,
				options.Force,
				options.SkipItems,
			); err != nil {
				return err
			}
		}

		if matchedCount == 0 {
			return faults.NewTypedError(
				faults.NotFoundError,
				fmt.Sprintf("no remote resources matched wildcard path %q", normalizedPath),
				nil,
			)
		}
		return nil
	}

	if !hasInput {
		remoteValue, err := resolveSaveRemoteValue(
			ctx,
			orchestratorService,
			deps.MetadataService(),
			normalizedPath,
			explicitCollectionTarget,
			options.SkipItems,
		)
		if err != nil {
			return err
		}
		value = remoteValue
	}

	return saveResolvedPathPayload(
		ctx,
		deps,
		orchestratorService,
		repositoryService,
		normalizedPath,
		value,
		options.AsItems,
		options.AsOneResource,
		options.Secret,
		options.AllowPlaintext,
		options.PruneDefaults,
		options.SecretAttributesEnabled,
		options.RequestedSecretAttributes,
		options.Force,
		options.SkipItems,
	)
}

func validateSecretAttributesPayloadType(
	descriptor resource.PayloadDescriptor,
	enabled bool,
) error {
	if !enabled {
		return nil
	}

	resolved := resource.NormalizePayloadDescriptor(descriptor)
	if resource.IsStructuredPayloadType(resolved.PayloadType) {
		return nil
	}

	return faults.NewValidationError(
		fmt.Sprintf(
			"--secret-attributes requires structured payload (json, yaml); got %q; use --secret for whole-resource secrets",
			resolved.PayloadType,
		),
		nil,
	)
}

func saveResolvedPathPayload(
	ctx context.Context,
	deps Dependencies,
	orchestratorService orchestratordomain.Orchestrator,
	repositoryService repository.ResourceStore,
	resolvedPath string,
	content resource.Content,
	asItems bool,
	asOneResource bool,
	secret bool,
	allowPlaintext bool,
	pruneDefaults bool,
	secretAttributesEnabled bool,
	requestedSecretAttributes []string,
	force bool,
	skipItems []string,
) error {
	items, isListPayload, err := extractSaveListItems(content.Value)
	if err != nil {
		return err
	}
	if err := validateSecretAttributesPayloadType(content.Descriptor, secretAttributesEnabled); err != nil {
		return err
	}

	autoWholeResourceSecret := false
	if !secret && !secretAttributesEnabled && !asItems {
		resolvedMetadata, err := resolveMetadataForSecretCheck(ctx, deps, resolvedPath)
		if err != nil {
			return err
		}
		autoWholeResourceSecret = resolvedMetadata.IsWholeResourceSecret()
	}

	if secret || autoWholeResourceSecret || asOneResource || (!asItems && !isListPayload) {
		if pruneDefaults {
			content, _, err = defaultsapp.CompactContentAgainstStoredDefaults(ctx, deps, resolvedPath, content)
			if err != nil {
				return err
			}
		}
		if err := ensureSaveTargetAllowed(ctx, repositoryService, resolvedPath, force); err != nil {
			return err
		}
		if secret || autoWholeResourceSecret {
			return saveResolvedPathAsSecret(ctx, deps, orchestratorService, resolvedPath, content)
		}
		value := content.Value
		if secretAttributesEnabled {
			value, unhandled, err := handleSaveSecrets(
				ctx,
				deps,
				resolvedPath,
				value,
				"",
				requestedSecretAttributes,
			)
			if err != nil {
				return err
			}
			declaredCandidates, err := resolveDeclaredSaveSecretAttributes(ctx, deps, resolvedPath)
			if err != nil {
				return err
			}
			blockingCandidates := filterSaveSecretCandidatesForSafety(unhandled, declaredCandidates, allowPlaintext)
			if len(blockingCandidates) > 0 {
				return saveSecretSafetyError(resolvedPath, blockingCandidates)
			}
			return orchestratorService.Save(ctx, resolvedPath, resource.Content{
				Value:      value,
				Descriptor: content.Descriptor,
			})
		}
		value, err = autoHandleDeclaredSaveSecrets(ctx, deps, resolvedPath, value)
		if err != nil {
			return err
		}
		if err := enforceSaveSecretSafety(ctx, deps, resolvedPath, value, allowPlaintext); err != nil {
			return err
		}
		return orchestratorService.Save(ctx, resolvedPath, resource.Content{
			Value:      value,
			Descriptor: content.Descriptor,
		})
	}
	if !isListPayload {
		return faults.NewValidationError("input payload is not a list; use --mode single to save a single resource", nil)
	}

	entries, err := resolveSaveEntriesForItems(ctx, deps, resolvedPath, items)
	if err != nil {
		return err
	}
	collectionFormat, err := resolveCollectionFormat(ctx, deps, resolvedPath)
	if err != nil {
		return err
	}
	for idx := range entries {
		switch {
		case metadatadomain.ResourceFormatAllowsMixedItems(collectionFormat):
			continue
		case strings.TrimSpace(collectionFormat) != "":
			entries[idx].Descriptor = resource.PayloadDescriptor{}
		case !resource.IsPayloadDescriptorExplicit(entries[idx].Descriptor):
			entries[idx].Descriptor = content.Descriptor
		}
	}
	if pruneDefaults {
		for idx := range entries {
			prunedContent, _, pruneErr := defaultsapp.CompactContentAgainstStoredDefaults(ctx, deps, entries[idx].LogicalPath, resource.Content{
				Value:      entries[idx].Payload,
				Descriptor: entries[idx].Descriptor,
			})
			if pruneErr != nil {
				return pruneErr
			}
			entries[idx].Payload = prunedContent.Value
			entries[idx].Descriptor = prunedContent.Descriptor
		}
	}
	entries = filterSaveEntriesForSkipItems(resolvedPath, entries, skipItems)
	if len(entries) == 0 {
		return nil
	}
	if err := ensureSaveEntriesWritable(ctx, repositoryService, entries, force); err != nil {
		return err
	}
	collectionCandidates, err := detectSaveSecretCandidatesForCollection(ctx, deps, resolvedPath, entries)
	if err != nil {
		return err
	}
	if secretAttributesEnabled {
		selectedCandidates, unhandledCandidates, err := selectSaveSecretCandidates(
			collectionCandidates,
			requestedSecretAttributes,
			true,
		)
		if err != nil {
			return err
		}

		if len(selectedCandidates) > 0 {
			secretProvider, err := appdeps.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			entries, err = applySaveSecretCandidatesToEntries(ctx, secretProvider, entries, selectedCandidates)
			if err != nil {
				return err
			}

			if err := persistSaveSecretAttributes(
				ctx,
				deps,
				saveSecretMetadataPathForCollection(resolvedPath),
				selectedCandidates,
			); err != nil {
				return err
			}
		}

		declaredCandidates, err := resolveDeclaredSaveSecretAttributes(ctx, deps, resolvedPath)
		if err != nil {
			return err
		}

		blockingCandidates := filterSaveSecretCandidatesForSafety(unhandledCandidates, declaredCandidates, allowPlaintext)
		if len(blockingCandidates) > 0 {
			return saveSecretSafetyError(resolvedPath, blockingCandidates)
		}
	} else {
		declaredCandidates, err := resolveDeclaredSaveSecretAttributes(ctx, deps, resolvedPath)
		if err != nil {
			return err
		}
		entries, err = autoHandleDeclaredSaveSecretsForEntries(
			ctx,
			deps,
			entries,
			collectionCandidates,
			declaredCandidates,
		)
		if err != nil {
			return err
		}

		blockingCandidates := filterSaveSecretCandidatesForSafety(collectionCandidates, declaredCandidates, allowPlaintext)
		if len(blockingCandidates) > 0 {
			return saveSecretSafetyError(resolvedPath, blockingCandidates)
		}
	}
	for _, entry := range entries {
		if err := orchestratorService.Save(ctx, entry.LogicalPath, resource.Content{
			Value:      entry.Payload,
			Descriptor: entry.Descriptor,
		}); err != nil {
			return err
		}
	}

	return nil
}

func resolveCollectionFormat(ctx context.Context, deps Dependencies, logicalPath string) (string, error) {
	resolvedMetadata, err := resolveMetadataForSecretCheck(ctx, deps, logicalPath)
	if err != nil {
		return "", err
	}
	return metadatadomain.NormalizeResourceFormat(resolvedMetadata.Format), nil
}
