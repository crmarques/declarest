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
	"errors"
	"fmt"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	"github.com/crmarques/declarest/resource"
)

type inferTemplateItem struct {
	logicalPath  string
	localAlias   string
	localContent resource.Content
}

func inferFromManagedService(
	ctx context.Context,
	deps Dependencies,
	resolvedTarget target,
	request InferRequest,
) (_ []resource.Value, err error) {
	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return nil, err
	}

	metadataService, err := appdeps.RequireMetadataService(deps)
	if err != nil {
		return nil, err
	}

	selectedItems, err := resolveInferTemplateItems(ctx, deps, resolvedTarget.scopePath, request.Items)
	if err != nil {
		return nil, err
	}

	probes := make([]managedServiceProbe, 0, len(selectedItems)*2)
	tempPaths := make([]string, 0, len(selectedItems)*2)
	defer func() {
		var cleanupErr error
		for idx := len(tempPaths) - 1; idx >= 0; idx-- {
			deleteErr := cleanupManagedServiceProbe(ctx, deps, orchestratorService, tempPaths[idx])
			if deleteErr != nil {
				cleanupErr = errors.Join(
					cleanupErr,
					faults.Invalid(
						fmt.Sprintf("failed to delete managed-service defaults probe %q", tempPaths[idx]),
						deleteErr,
					),
				)
			}
		}
		if cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()

	for _, item := range selectedItems {
		md, metadataErr := metadataService.ResolveForPath(ctx, item.logicalPath)
		if metadataErr != nil {
			return nil, metadataErr
		}

		rawContent, rawErr := resolveManagedServiceProbeContent(ctx, deps, item.logicalPath)
		if rawErr != nil {
			return nil, rawErr
		}
		if !resource.IsPayloadDescriptorExplicit(rawContent.Descriptor) && resource.IsPayloadDescriptorExplicit(item.localContent.Descriptor) {
			rawContent.Descriptor = item.localContent.Descriptor
		}

		firstPayload, firstPath, buildErr := buildManagedServiceProbePayload(item.logicalPath, md, rawContent, "probe-1")
		if buildErr != nil {
			return nil, buildErr
		}
		secondPayload, secondPath, buildErr := buildManagedServiceProbePayload(item.logicalPath, md, rawContent, "probe-2")
		if buildErr != nil {
			return nil, buildErr
		}

		if _, createErr := orchestratorService.Create(ctx, firstPath, firstPayload); createErr != nil {
			return nil, createErr
		}
		tempPaths = append(tempPaths, firstPath)
		probes = append(probes, managedServiceProbe{path: firstPath})

		if _, createErr := orchestratorService.Create(ctx, secondPath, secondPayload); createErr != nil {
			return nil, createErr
		}
		tempPaths = append(tempPaths, secondPath)
		probes = append(probes, managedServiceProbe{path: secondPath})
	}

	if request.Wait > 0 {
		if err := waitForManagedServiceDelay(ctx, request.Wait); err != nil {
			return nil, err
		}
	}

	invalidateManagedServiceAuthCache(deps)

	outputs := make([]resource.Value, 0, len(probes))
	for _, probe := range probes {
		remoteContent, readErr := readManagedServiceProbeContent(ctx, orchestratorService, probe.path)
		if readErr != nil {
			return nil, readErr
		}
		outputs = append(outputs, remoteContent.Value)
	}
	return outputs, nil
}
