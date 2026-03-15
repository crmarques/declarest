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

package detect

import (
	"context"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	secretworkflow "github.com/crmarques/declarest/internal/app/secret/workflow"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type Dependencies = appdeps.Dependencies

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
	secretProvider, err := appdeps.RequireSecretProvider(deps)
	if err != nil {
		return Result{}, err
	}

	if req.HasInput {
		keys, err := secretProvider.DetectSecretCandidates(ctx, req.Value)
		if err != nil {
			return Result{}, err
		}

		appliedKeys, err := resolveDetectSecretAttributes(keys, req.SecretAttribute)
		if err != nil {
			return Result{}, err
		}

		if req.Fix {
			if strings.TrimSpace(req.ResolvedPath) == "" {
				return Result{}, faults.NewValidationError("path is required", nil)
			}
			if err := applyDetectedSecretAttributes(ctx, deps, req.ResolvedPath, appliedKeys); err != nil {
				return Result{}, err
			}
		} else if strings.TrimSpace(req.ResolvedPath) != "" {
			return Result{}, faults.NewValidationError("path input requires --fix when detecting from input payload", nil)
		}

		return Result{Output: appliedKeys}, nil
	}

	scanPath := strings.TrimSpace(req.ResolvedPath)
	if scanPath == "" {
		scanPath = "/"
	}

	results, err := detectSecretCandidatesFromRepository(ctx, deps, secretProvider, scanPath, req.SecretAttribute)
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
	secretProvider secretdomain.SecretProvider,
	scanPath string,
	secretAttribute string,
) ([]DetectedResourceSecrets, error) {
	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return nil, err
	}

	items, err := orchestratorService.ListLocal(ctx, scanPath, orchestratordomain.ListPolicy{Recursive: true})
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

		content, err := orchestratorService.GetLocal(ctx, item.LogicalPath)
		if err != nil {
			return nil, err
		}

		keys, err := secretProvider.DetectSecretCandidates(ctx, content.Value)
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
		return nil, faults.NewValidationError("requested --secret-attribute was not detected", nil)
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
		return nil, faults.NewValidationError("requested --secret-attribute was not detected", nil)
	}

	return []string{}, nil
}

func applyDetectedSecretAttributes(ctx context.Context, deps Dependencies, logicalPath string, detected []string) error {
	if len(detected) == 0 {
		return nil
	}
	metadataService, err := appdeps.RequireMetadataService(deps)
	if err != nil {
		return err
	}
	return secretworkflow.PersistDetectedAttributes(ctx, metadataService, logicalPath, detected)
}
