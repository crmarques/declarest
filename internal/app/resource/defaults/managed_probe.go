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
	"net/http"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	managedservicedomain "github.com/crmarques/declarest/managedservice"
	"github.com/crmarques/declarest/metadata"
	metadatavalidation "github.com/crmarques/declarest/metadata/validation"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/resource/identity"
)

type managedServiceProbe struct {
	path string
}

func resolveManagedServiceProbeContent(ctx context.Context, deps Dependencies, logicalPath string) (resource.Content, error) {
	store, err := appdeps.RequireResourceStore(deps)
	if err != nil {
		return resource.Content{}, err
	}

	return store.Get(ctx, logicalPath)
}

func invalidateManagedServiceAuthCache(deps Dependencies) {
	if deps.Services == nil {
		return
	}
	managedServiceClient := deps.Services.ManagedServiceClient()
	if managedServiceClient == nil {
		return
	}
	invalidator, ok := managedServiceClient.(managedServiceAuthCacheInvalidator)
	if !ok {
		return
	}
	invalidator.InvalidateAuthCache()
}

func readManagedServiceProbeContent(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	logicalPath string,
) (resource.Content, error) {
	var (
		lastContent    resource.Content
		lastNormalized resource.Value
		stableReads    int
	)

	for attempt := 0; attempt < managedServiceProbeReadAttempts; attempt++ {
		content, err := orchestratorService.GetRemote(ctx, logicalPath)
		if err != nil {
			return resource.Content{}, err
		}

		normalized, err := resource.Normalize(content.Value)
		if err != nil {
			return resource.Content{}, err
		}
		content.Value = normalized

		if attempt > 0 && reflect.DeepEqual(normalized, lastNormalized) {
			stableReads++
		} else {
			stableReads = 1
		}

		lastContent = content
		lastNormalized = normalized

		if attempt+1 >= managedServiceProbeReadMinAttempts && stableReads >= 2 {
			return lastContent, nil
		}
		if attempt+1 == managedServiceProbeReadAttempts {
			break
		}
		if waitErr := waitForManagedServiceDelay(ctx, managedServiceProbeReadDelay); waitErr != nil {
			return resource.Content{}, waitErr
		}
	}

	return lastContent, nil
}

func cleanupManagedServiceProbe(
	ctx context.Context,
	deps Dependencies,
	orchestratorService orchestratordomain.Orchestrator,
	logicalPath string,
) error {
	deleteErr := orchestratorService.Delete(ctx, logicalPath, orchestratordomain.DeletePolicy{})
	if deleteErr == nil || faults.IsCategory(deleteErr, faults.NotFoundError) {
		return nil
	}
	if !faults.IsCategory(deleteErr, faults.AuthError) {
		return deleteErr
	}

	retryErr := retryManagedServiceProbeDelete(ctx, deps, logicalPath)
	if retryErr == nil || faults.IsCategory(retryErr, faults.NotFoundError) {
		return nil
	}
	return errors.Join(deleteErr, retryErr)
}

func retryManagedServiceProbeDelete(ctx context.Context, deps Dependencies, logicalPath string) error {
	if deps.Services == nil {
		return faults.Invalid("managed-service cleanup retry requires service accessor", nil)
	}
	managedServiceClient := deps.Services.ManagedServiceClient()
	if managedServiceClient == nil {
		return faults.Invalid("managed-service cleanup retry requires managed-service client", nil)
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if invalidator, ok := managedServiceClient.(managedServiceAuthCacheInvalidator); ok {
			invalidator.InvalidateAuthCache()
		}
		_, err := managedServiceClient.Request(ctx, managedservicedomain.RequestSpec{
			Method: http.MethodDelete,
			Path:   logicalPath,
		})
		if err == nil || faults.IsCategory(err, faults.NotFoundError) {
			return nil
		}
		lastErr = err
		if !faults.IsCategory(err, faults.AuthError) || attempt == 1 {
			break
		}
		if waitErr := waitForManagedServiceDelay(ctx, 250*time.Millisecond); waitErr != nil {
			return errors.Join(lastErr, waitErr)
		}
	}
	return lastErr
}

func waitForManagedServiceDelay(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func buildManagedServiceProbePayload(
	logicalPath string,
	md metadata.ResourceMetadata,
	content resource.Content,
	label string,
) (resource.Content, string, error) {
	normalizedValue, err := resource.Normalize(content.Value)
	if err != nil {
		return resource.Content{}, "", err
	}

	payload, ok := normalizedValue.(map[string]any)
	if !ok {
		return resource.Content{}, "", faults.Invalid("managed-service defaults inference requires an object payload", nil)
	}

	requiredAttributes, err := metadatavalidation.EffectiveCreatePayloadRequiredAttributes(md)
	if err != nil {
		return resource.Content{}, "", err
	}
	nextPayload, selectedPointers, err := selectManagedServiceProbePayload(payload, requiredAttributes)
	if err != nil {
		return resource.Content{}, "", err
	}

	aliasPointer, aliasOK, err := identity.SimpleAliasPointer(md)
	if err != nil {
		return resource.Content{}, "", err
	}
	if !aliasOK && strings.TrimSpace(md.Alias) == "" {
		aliasPointer = "/id"
		aliasOK = true
	}

	idPointer, idOK, err := identity.SimpleIDPointer(md)
	if err != nil {
		return resource.Content{}, "", err
	}
	if !idOK && strings.TrimSpace(md.ID) == "" {
		idPointer = "/id"
		idOK = true
	}

	if !aliasOK && !idOK {
		return resource.Content{}, "", faults.Invalid(
			"managed-service defaults inference requires simple resource.alias or resource.id metadata",
			nil,
		)
	}

	tempName := "declarest-defaults-" + label + "-" + strings.ToLower(uuid.NewString()[:8])
	next := any(nextPayload)
	replacedPointers := map[string]struct{}{}

	if aliasOK {
		if _, selected := selectedPointers[aliasPointer]; selected {
			next, err = resource.SetJSONPointerValue(next, aliasPointer, tempName)
			if err != nil {
				return resource.Content{}, "", err
			}
			replacedPointers[aliasPointer] = struct{}{}
		}
	}
	if idOK {
		if _, selected := selectedPointers[idPointer]; selected {
			if _, replaced := replacedPointers[idPointer]; !replaced {
				next, err = resource.SetJSONPointerValue(next, idPointer, tempName)
				if err != nil {
					return resource.Content{}, "", err
				}
				replacedPointers[idPointer] = struct{}{}
			}
		}
	}

	rewrittenPayload, ok := next.(map[string]any)
	if !ok {
		return resource.Content{}, "", faults.Invalid("managed-service defaults inference requires an object payload", nil)
	}
	if len(selectedPointers) == 0 {
		rewrittenPayload, err = applyManagedServiceProbeIdentityFallback(logicalPath, payload, rewrittenPayload, tempName, replacedPointers, aliasPointer, idPointer)
		if err != nil {
			return resource.Content{}, "", err
		}
	}

	return resource.Content{
		Value:      rewrittenPayload,
		Descriptor: content.Descriptor,
	}, joinLogicalPath(collectionPathFor(logicalPath), tempName), nil
}

func selectManagedServiceProbePayload(
	payload map[string]any,
	requiredAttributes []string,
) (map[string]any, map[string]struct{}, error) {
	selectedPayload := map[string]any{}
	selectedPointers := map[string]struct{}{}

	pointers, err := metadatavalidation.NormalizeAttributePointers(
		"managed-service defaults inference create required attributes",
		requiredAttributes,
	)
	if err != nil {
		return nil, nil, err
	}

	for _, pointer := range pointers {
		selectedPointers[pointer] = struct{}{}

		value, found, err := resource.LookupJSONPointer(payload, pointer)
		if err != nil {
			return nil, nil, err
		}
		if !found || value == nil {
			continue
		}

		next, err := resource.SetJSONPointerValue(selectedPayload, pointer, resource.DeepCopyValue(value))
		if err != nil {
			return nil, nil, err
		}

		typed, ok := next.(map[string]any)
		if !ok {
			return nil, nil, faults.Invalid("managed-service defaults inference requires an object payload", nil)
		}
		selectedPayload = typed
	}

	return selectedPayload, selectedPointers, nil
}

func applyManagedServiceProbeIdentityFallback(
	logicalPath string,
	originalPayload map[string]any,
	nextPayload map[string]any,
	tempName string,
	replacedPointers map[string]struct{},
	identityPointers ...string,
) (map[string]any, error) {
	identityValues, err := managedServiceProbeIdentityValues(logicalPath, originalPayload, identityPointers...)
	if err != nil {
		return nil, err
	}
	if len(identityValues) == 0 {
		return nextPayload, nil
	}

	allowedKeys := managedServiceProbeIdentityFieldKeys(logicalPath)
	current := nextPayload
	for key, rawValue := range originalPayload {
		value, ok := rawValue.(string)
		if !ok || !matchesManagedServiceProbeIdentityValue(identityValues, value) {
			continue
		}

		pointer := resource.JSONPointerForObjectKey(key)
		if _, alreadyReplaced := replacedPointers[pointer]; alreadyReplaced {
			continue
		}
		if _, allowed := allowedKeys[canonicalManagedServiceProbeFieldKey(key)]; !allowed {
			continue
		}

		updated, err := resource.SetJSONPointerValue(current, pointer, tempName)
		if err != nil {
			return nil, err
		}
		objectValue, ok := updated.(map[string]any)
		if !ok {
			return nil, faults.Invalid("managed-service defaults inference requires an object payload", nil)
		}
		current = objectValue
	}

	return current, nil
}

func managedServiceProbeIdentityValues(
	logicalPath string,
	payload map[string]any,
	identityPointers ...string,
) (map[string]struct{}, error) {
	values := map[string]struct{}{}
	addManagedServiceProbeIdentityValue(values, path.Base(strings.TrimSpace(logicalPath)))

	for _, pointer := range identityPointers {
		value, found, err := resource.LookupJSONPointerString(payload, pointer)
		if err != nil {
			return nil, err
		}
		if found {
			addManagedServiceProbeIdentityValue(values, value)
		}
	}

	return values, nil
}

func addManagedServiceProbeIdentityValue(values map[string]struct{}, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "/" || trimmed == "." {
		return
	}
	values[trimmed] = struct{}{}
}

func matchesManagedServiceProbeIdentityValue(values map[string]struct{}, value string) bool {
	_, ok := values[strings.TrimSpace(value)]
	return ok
}

func managedServiceProbeIdentityFieldKeys(logicalPath string) map[string]struct{} {
	keys := map[string]struct{}{}
	for _, candidate := range []string{"id", "name", "slug", "key", "code", "alias", "identifier", "uid"} {
		addManagedServiceProbeIdentityFieldKey(keys, candidate)
	}

	collectionSegments := resource.SplitLogicalPathSegments(collectionPathFor(logicalPath))
	if len(collectionSegments) == 0 {
		return keys
	}

	collectionName := collectionSegments[len(collectionSegments)-1]
	singularName := singularManagedServiceProbeIdentityField(collectionName)
	for _, candidate := range []string{
		collectionName,
		singularName,
		singularName + "id",
		singularName + "name",
	} {
		addManagedServiceProbeIdentityFieldKey(keys, candidate)
	}
	return keys
}

func addManagedServiceProbeIdentityFieldKey(keys map[string]struct{}, value string) {
	canonical := canonicalManagedServiceProbeFieldKey(value)
	if canonical == "" {
		return
	}
	keys[canonical] = struct{}{}
}
