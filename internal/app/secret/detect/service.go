package detect

import (
	"context"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	secretworkflow "github.com/crmarques/declarest/internal/app/secret/workflow"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type Dependencies struct {
	Orchestrator   orchestratordomain.Orchestrator
	Metadata       metadatadomain.MetadataService
	SecretProvider secretdomain.SecretProvider
}

type Request struct {
	ResolvedPath    string
	Value           resource.Value
	HasInput        bool
	Fix             bool
	SecretAttribute string
}

type Result struct {
	Output any
}

type DetectedResourceSecrets struct {
	LogicalPath string
	Attributes  []string
}

func Execute(ctx context.Context, deps Dependencies, req Request) (Result, error) {
	if deps.SecretProvider == nil {
		return Result{}, validationError("secret provider is not configured", nil)
	}

	if req.HasInput {
		keys, err := deps.SecretProvider.DetectSecretCandidates(ctx, req.Value)
		if err != nil {
			return Result{}, err
		}

		appliedKeys, err := resolveDetectSecretAttributes(keys, req.SecretAttribute)
		if err != nil {
			return Result{}, err
		}

		if req.Fix {
			if strings.TrimSpace(req.ResolvedPath) == "" {
				return Result{}, validationError("path is required", nil)
			}
			if err := applyDetectedSecretAttributes(ctx, deps, req.ResolvedPath, appliedKeys); err != nil {
				return Result{}, err
			}
		} else if strings.TrimSpace(req.ResolvedPath) != "" {
			return Result{}, validationError("path input requires --fix when detecting from input payload", nil)
		}

		return Result{Output: appliedKeys}, nil
	}

	scanPath := strings.TrimSpace(req.ResolvedPath)
	if scanPath == "" {
		scanPath = "/"
	}

	results, err := detectSecretCandidatesFromRepository(ctx, deps, scanPath, req.SecretAttribute)
	if err != nil {
		return Result{}, err
	}

	if req.Fix {
		for _, result := range results {
			if err := applyDetectedSecretAttributes(ctx, deps, result.LogicalPath, result.Attributes); err != nil {
				return Result{}, err
			}
		}
	}

	return Result{Output: results}, nil
}

func detectSecretCandidatesFromRepository(
	ctx context.Context,
	deps Dependencies,
	scanPath string,
	secretAttribute string,
) ([]DetectedResourceSecrets, error) {
	if deps.Orchestrator == nil {
		return nil, validationError("orchestrator is not configured", nil)
	}

	items, err := deps.Orchestrator.ListLocal(ctx, scanPath, orchestratordomain.ListPolicy{Recursive: true})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})

	results := make([]DetectedResourceSecrets, 0, len(items))
	requestedAttribute := strings.TrimSpace(secretAttribute)
	requestedAttributeMatched := false

	for _, item := range items {
		if strings.TrimSpace(item.LogicalPath) == "" {
			continue
		}

		value, err := deps.Orchestrator.GetLocal(ctx, item.LogicalPath)
		if err != nil {
			return nil, err
		}

		keys, err := deps.SecretProvider.DetectSecretCandidates(ctx, value)
		if err != nil {
			return nil, err
		}

		filtered, matched := filterDetectedSecretAttributes(keys, requestedAttribute)
		if !matched {
			continue
		}
		if requestedAttribute != "" {
			requestedAttributeMatched = true
		}

		results = append(results, DetectedResourceSecrets{
			LogicalPath: item.LogicalPath,
			Attributes:  filtered,
		})
	}

	if requestedAttribute != "" && !requestedAttributeMatched {
		return nil, validationError("requested --secret-attribute was not detected", nil)
	}

	return results, nil
}

func filterDetectedSecretAttributes(keys []string, secretAttribute string) ([]string, bool) {
	normalizedKeys := secretworkflow.DedupeAndSortAttributes(keys)
	if len(normalizedKeys) == 0 {
		return nil, false
	}

	attribute := strings.TrimSpace(secretAttribute)
	if attribute == "" {
		return normalizedKeys, true
	}

	for _, key := range normalizedKeys {
		if key == attribute {
			return []string{attribute}, true
		}
	}
	return nil, false
}

func resolveDetectSecretAttributes(keys []string, secretAttribute string) ([]string, error) {
	filtered, matched := filterDetectedSecretAttributes(keys, secretAttribute)
	if matched {
		return filtered, nil
	}

	if strings.TrimSpace(secretAttribute) != "" {
		return nil, validationError("requested --secret-attribute was not detected", nil)
	}

	return []string{}, nil
}

func applyDetectedSecretAttributes(ctx context.Context, deps Dependencies, logicalPath string, detected []string) error {
	if len(detected) == 0 {
		return nil
	}
	if deps.Metadata == nil {
		return validationError("metadata service is not configured", nil)
	}
	return secretworkflow.PersistDetectedAttributes(ctx, deps.Metadata, logicalPath, detected)
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}
