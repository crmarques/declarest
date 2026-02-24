package save

import (
	"context"
	"fmt"

	"github.com/crmarques/declarest/faults"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type Dependencies struct {
	Orchestrator orchestratordomain.Orchestrator
	Repository   repository.ResourceStore
	Metadata     metadatadomain.MetadataService
	Secrets      secretdomain.SecretProvider
}

type ExecuteOptions struct {
	AsItems       bool
	AsOneResource bool
	Ignore        bool
	Force         bool

	HandleSecretsEnabled      bool
	RequestedSecretCandidates []string
}

type saveRemoteReader interface {
	GetRemote(ctx context.Context, logicalPath string) (resource.Value, error)
	ListRemote(ctx context.Context, logicalPath string, policy orchestratordomain.ListPolicy) ([]resource.Resource, error)
}

func Execute(
	ctx context.Context,
	deps Dependencies,
	resolvedPath string,
	value resource.Value,
	hasInput bool,
	options ExecuteOptions,
) error {
	normalizedPath, hasWildcard, explicitCollectionTarget, err := normalizeSavePathPattern(resolvedPath)
	if err != nil {
		return err
	}
	if options.AsItems && options.AsOneResource {
		return validationError("flags --as-items and --as-one-resource cannot be used together", nil)
	}

	orchestratorService, err := requireOrchestrator(deps)
	if err != nil {
		return err
	}
	repositoryService, err := requireResourceStore(deps)
	if err != nil {
		return err
	}

	if hasWildcard {
		if hasInput {
			return validationError("wildcard save paths are supported only when reading from remote server", nil)
		}

		targets, err := expandSaveWildcardPaths(ctx, orchestratorService, normalizedPath)
		if err != nil {
			return err
		}

		matchedCount := 0
		for _, targetPath := range targets {
			remoteValue, err := orchestratorService.GetRemote(ctx, targetPath)
			if err != nil {
				if isTypedErrorCategory(err, faults.NotFoundError) {
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
				options.Ignore,
				options.HandleSecretsEnabled,
				options.RequestedSecretCandidates,
				options.Force,
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
			deps.Metadata,
			normalizedPath,
			explicitCollectionTarget,
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
		options.Ignore,
		options.HandleSecretsEnabled,
		options.RequestedSecretCandidates,
		options.Force,
	)
}
func saveResolvedPathPayload(
	ctx context.Context,
	deps Dependencies,
	orchestratorService orchestratordomain.Orchestrator,
	repositoryService repository.ResourceStore,
	resolvedPath string,
	value resource.Value,
	asItems bool,
	asOneResource bool,
	ignore bool,
	handleSecretsEnabled bool,
	requestedSecretCandidates []string,
	force bool,
) error {
	items, isListPayload, err := extractSaveListItems(value)
	if err != nil {
		return err
	}

	if asOneResource || (!asItems && !isListPayload) {
		if err := ensureSaveTargetAllowed(ctx, repositoryService, resolvedPath, force); err != nil {
			return err
		}
		if handleSecretsEnabled {
			value, unhandled, err := handleSaveSecrets(
				ctx,
				deps,
				resolvedPath,
				value,
				"",
				requestedSecretCandidates,
			)
			if err != nil {
				return err
			}
			declaredCandidates, err := resolveDeclaredSaveSecretAttributes(ctx, deps, resolvedPath)
			if err != nil {
				return err
			}
			blockingCandidates := filterSaveSecretCandidatesForSafety(unhandled, declaredCandidates, ignore)
			if len(blockingCandidates) > 0 {
				return saveSecretSafetyError(resolvedPath, blockingCandidates)
			}
			return orchestratorService.Save(ctx, resolvedPath, value)
		}
		value, err = autoHandleDeclaredSaveSecrets(ctx, deps, resolvedPath, value)
		if err != nil {
			return err
		}
		if err := enforceSaveSecretSafety(ctx, deps, resolvedPath, value, ignore); err != nil {
			return err
		}
		return orchestratorService.Save(ctx, resolvedPath, value)
	}
	if !isListPayload {
		return validationError("input payload is not a list; use --as-one-resource to save a single resource", nil)
	}

	entries, err := resolveSaveEntriesForItems(ctx, deps, resolvedPath, items)
	if err != nil {
		return err
	}
	if err := ensureSaveEntriesWritable(ctx, repositoryService, entries, force); err != nil {
		return err
	}
	collectionCandidates, err := detectSaveSecretCandidatesForCollection(ctx, deps, resolvedPath, entries)
	if err != nil {
		return err
	}
	if handleSecretsEnabled {
		selectedCandidates, unhandledCandidates, err := selectSaveSecretCandidates(
			collectionCandidates,
			requestedSecretCandidates,
			true,
		)
		if err != nil {
			return err
		}

		if len(selectedCandidates) > 0 {
			secretProvider, err := requireSecretProvider(deps)
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

		blockingCandidates := filterSaveSecretCandidatesForSafety(unhandledCandidates, declaredCandidates, ignore)
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

		blockingCandidates := filterSaveSecretCandidatesForSafety(collectionCandidates, declaredCandidates, ignore)
		if len(blockingCandidates) > 0 {
			return saveSecretSafetyError(resolvedPath, blockingCandidates)
		}
	}
	for _, entry := range entries {
		if err := orchestratorService.Save(ctx, entry.LogicalPath, entry.Payload); err != nil {
			return err
		}
	}

	return nil
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}

func requireOrchestrator(deps Dependencies) (orchestratordomain.Orchestrator, error) {
	if deps.Orchestrator == nil {
		return nil, validationError("orchestrator is not configured", nil)
	}
	return deps.Orchestrator, nil
}

func requireResourceStore(deps Dependencies) (repository.ResourceStore, error) {
	if deps.Repository == nil {
		return nil, validationError("resource repository is not configured", nil)
	}
	return deps.Repository, nil
}

func requireMetadataService(deps Dependencies) (metadatadomain.MetadataService, error) {
	if deps.Metadata == nil {
		return nil, validationError("metadata service is not configured", nil)
	}
	return deps.Metadata, nil
}

func requireSecretProvider(deps Dependencies) (secretdomain.SecretProvider, error) {
	if deps.Secrets == nil {
		return nil, validationError("secret provider is not configured", nil)
	}
	return deps.Secrets, nil
}
