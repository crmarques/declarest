package reconciler

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/server"
)

var _ ResourceReconciler = (*DefaultReconciler)(nil)

type DefaultReconciler struct {
	Name              string
	RepositoryManager repository.ResourceRepositoryManager
	MetadataService   metadata.MetadataService
	ServerManager     server.ResourceServerManager
	SecretsProvider   secrets.SecretProvider
}

func (r *DefaultReconciler) MetadataManager() metadata.MetadataService {
	if r == nil {
		return nil
	}
	return r.MetadataService
}

func (r *DefaultReconciler) SecretManager() secrets.SecretProvider {
	if r == nil {
		return nil
	}
	return r.SecretsProvider
}

func (r *DefaultReconciler) Get(ctx context.Context, logicalPath string) (resource.Value, error) {
	manager, err := r.repositoryManager()
	if err != nil {
		return nil, err
	}
	return manager.Get(ctx, logicalPath)
}

func (r *DefaultReconciler) Save(ctx context.Context, logicalPath string, value resource.Value) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Save(ctx, logicalPath, value)
}

func (r *DefaultReconciler) Apply(ctx context.Context, logicalPath string) (resource.Resource, error) {
	manager, err := r.repositoryManager()
	if err != nil {
		return resource.Resource{}, err
	}

	localValue, err := manager.Get(ctx, logicalPath)
	if err != nil {
		return resource.Resource{}, err
	}

	resourceInfo, err := r.buildResourceInfo(ctx, logicalPath, localValue)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.Payload)
	if err != nil {
		return resource.Resource{}, err
	}
	resourceInfo.Payload = resolvedPayload

	serverManager, err := r.serverManager()
	if err != nil {
		return resource.Resource{}, err
	}

	exists, err := serverManager.Exists(ctx, resourceInfo)
	if err != nil {
		return resource.Resource{}, err
	}

	operation := metadata.OperationCreate
	if exists {
		operation = metadata.OperationUpdate
	}

	return r.executeRemoteMutation(ctx, resourceInfo, operation)
}

func (r *DefaultReconciler) Create(ctx context.Context, logicalPath string, value resource.Value) (resource.Resource, error) {
	resourceInfo, err := r.buildResourceInfo(ctx, logicalPath, value)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.Payload)
	if err != nil {
		return resource.Resource{}, err
	}
	resourceInfo.Payload = resolvedPayload

	return r.executeRemoteMutation(ctx, resourceInfo, metadata.OperationCreate)
}

func (r *DefaultReconciler) Update(ctx context.Context, logicalPath string, value resource.Value) (resource.Resource, error) {
	resourceInfo, err := r.buildResourceInfo(ctx, logicalPath, value)
	if err != nil {
		return resource.Resource{}, err
	}

	resolvedPayload, err := r.resolvePayloadForRemote(ctx, resourceInfo.Payload)
	if err != nil {
		return resource.Resource{}, err
	}
	resourceInfo.Payload = resolvedPayload

	return r.executeRemoteMutation(ctx, resourceInfo, metadata.OperationUpdate)
}

func (r *DefaultReconciler) Delete(ctx context.Context, logicalPath string, policy DeletePolicy) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Delete(ctx, logicalPath, repository.DeletePolicy{Recursive: policy.Recursive})
}

func (r *DefaultReconciler) ListLocal(ctx context.Context, logicalPath string, policy ListPolicy) ([]resource.Resource, error) {
	manager, err := r.repositoryManager()
	if err != nil {
		return nil, err
	}
	return manager.List(ctx, logicalPath, repository.ListPolicy{Recursive: policy.Recursive})
}

func (r *DefaultReconciler) ListRemote(ctx context.Context, logicalPath string, policy ListPolicy) ([]resource.Resource, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}

	metadataService, err := r.metadataService()
	if err != nil {
		return nil, err
	}
	serverManager, err := r.serverManager()
	if err != nil {
		return nil, err
	}

	resolvedMetadata, err := metadataService.ResolveForPath(ctx, normalizedPath)
	if err != nil {
		return nil, err
	}

	items, err := serverManager.List(ctx, normalizedPath, resolvedMetadata)
	if err != nil {
		return nil, err
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})

	if policy.Recursive {
		return items, nil
	}

	direct := make([]resource.Resource, 0, len(items))
	for _, item := range items {
		if isDirectChildPath(normalizedPath, item.LogicalPath) {
			direct = append(direct, item)
		}
	}
	return direct, nil
}

func (r *DefaultReconciler) Explain(ctx context.Context, logicalPath string) ([]resource.DiffEntry, error) {
	return r.Diff(ctx, logicalPath)
}

func (r *DefaultReconciler) Diff(ctx context.Context, logicalPath string) ([]resource.DiffEntry, error) {
	manager, err := r.repositoryManager()
	if err != nil {
		return nil, err
	}

	localValue, err := manager.Get(ctx, logicalPath)
	if err != nil {
		return nil, err
	}

	resourceInfo, err := r.buildResourceInfo(ctx, logicalPath, localValue)
	if err != nil {
		return nil, err
	}

	localForCompare, err := r.resolvePayloadForRemote(ctx, resourceInfo.Payload)
	if err != nil {
		return nil, err
	}
	resourceInfo.Payload = localForCompare

	remoteValue, err := r.fetchRemoteValue(ctx, resourceInfo)
	if err != nil {
		return nil, err
	}

	compareSpec, err := r.renderOperationSpec(ctx, resourceInfo, metadata.OperationCompare, localForCompare)
	if err != nil {
		return nil, err
	}

	localTransformed, err := applyCompareTransforms(localForCompare, compareSpec)
	if err != nil {
		return nil, err
	}
	remoteTransformed, err := applyCompareTransforms(remoteValue, compareSpec)
	if err != nil {
		return nil, err
	}

	items := buildDiffEntries(resourceInfo.LogicalPath, localTransformed, remoteTransformed)
	sort.Slice(items, func(i int, j int) bool {
		if items[i].Path == items[j].Path {
			return items[i].Operation < items[j].Operation
		}
		return items[i].Path < items[j].Path
	})
	return items, nil
}

func (r *DefaultReconciler) Template(ctx context.Context, logicalPath string, value resource.Value) (resource.Value, error) {
	resourceInfo, err := r.buildResourceInfo(ctx, logicalPath, value)
	if err != nil {
		return nil, err
	}

	spec, err := r.renderOperationSpec(ctx, resourceInfo, metadata.OperationUpdate, resourceInfo.Payload)
	if err != nil {
		return nil, err
	}

	if spec.Body != nil {
		return resource.Normalize(spec.Body)
	}

	return resource.Normalize(resourceInfo.Payload)
}

func (r *DefaultReconciler) RepoInit(ctx context.Context) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Init(ctx)
}

func (r *DefaultReconciler) RepoRefresh(ctx context.Context) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Refresh(ctx)
}

func (r *DefaultReconciler) RepoPush(ctx context.Context, policy repository.PushPolicy) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Push(ctx, policy)
}

func (r *DefaultReconciler) RepoReset(ctx context.Context, policy repository.ResetPolicy) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Reset(ctx, policy)
}

func (r *DefaultReconciler) RepoCheck(ctx context.Context) error {
	manager, err := r.repositoryManager()
	if err != nil {
		return err
	}
	return manager.Check(ctx)
}

func (r *DefaultReconciler) RepoStatus(ctx context.Context) (repository.SyncReport, error) {
	manager, err := r.repositoryManager()
	if err != nil {
		return repository.SyncReport{}, err
	}
	return manager.SyncStatus(ctx)
}

func (r *DefaultReconciler) repositoryManager() (repository.ResourceRepositoryManager, error) {
	if r == nil || r.RepositoryManager == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "repository manager is not configured", nil)
	}
	return r.RepositoryManager, nil
}

func (r *DefaultReconciler) metadataService() (metadata.MetadataService, error) {
	if r == nil || r.MetadataService == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "metadata service is not configured", nil)
	}
	return r.MetadataService, nil
}

func (r *DefaultReconciler) serverManager() (server.ResourceServerManager, error) {
	if r == nil || r.ServerManager == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "server manager is not configured", nil)
	}
	return r.ServerManager, nil
}

func (r *DefaultReconciler) buildResourceInfo(
	ctx context.Context,
	logicalPath string,
	value resource.Value,
) (resource.Resource, error) {
	metadataService, err := r.metadataService()
	if err != nil {
		return resource.Resource{}, err
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return resource.Resource{}, err
	}

	collectionPath := "/"
	if normalizedPath != "/" {
		collectionPath = path.Dir(normalizedPath)
		if collectionPath == "." || collectionPath == "" {
			collectionPath = "/"
		}
	}

	resolvedMetadata, err := metadataService.ResolveForPath(ctx, normalizedPath)
	if err != nil {
		return resource.Resource{}, err
	}

	normalizedPayload, err := resource.Normalize(value)
	if err != nil {
		return resource.Resource{}, err
	}

	localAlias := path.Base(normalizedPath)
	if normalizedPath == "/" {
		localAlias = "/"
	}

	remoteID := localAlias
	if payloadMap, ok := normalizedPayload.(map[string]any); ok {
		if aliasAttribute := strings.TrimSpace(resolvedMetadata.AliasFromAttribute); aliasAttribute != "" {
			if aliasValue, found := payloadMap[aliasAttribute]; found {
				alias := strings.TrimSpace(fmt.Sprint(aliasValue))
				if alias != "" {
					localAlias = alias
				}
			}
		}

		if idAttribute := strings.TrimSpace(resolvedMetadata.IDFromAttribute); idAttribute != "" {
			if idValue, found := payloadMap[idAttribute]; found {
				id := strings.TrimSpace(fmt.Sprint(idValue))
				if id != "" {
					remoteID = id
				}
			}
		} else {
			remoteID = localAlias
		}
	}

	if strings.TrimSpace(localAlias) == "" {
		localAlias = path.Base(normalizedPath)
	}
	if strings.TrimSpace(remoteID) == "" {
		remoteID = localAlias
	}

	return resource.Resource{
		LogicalPath:    normalizedPath,
		CollectionPath: collectionPath,
		LocalAlias:     localAlias,
		RemoteID:       remoteID,
		Metadata:       resolvedMetadata,
		Payload:        normalizedPayload,
	}, nil
}

func (r *DefaultReconciler) executeRemoteMutation(
	ctx context.Context,
	resourceInfo resource.Resource,
	operation metadata.Operation,
) (resource.Resource, error) {
	manager, err := r.repositoryManager()
	if err != nil {
		return resource.Resource{}, err
	}
	serverManager, err := r.serverManager()
	if err != nil {
		return resource.Resource{}, err
	}

	var remotePayload resource.Value
	switch operation {
	case metadata.OperationCreate:
		remotePayload, err = serverManager.Create(ctx, resourceInfo)
	case metadata.OperationUpdate:
		remotePayload, err = serverManager.Update(ctx, resourceInfo)
	default:
		return resource.Resource{}, faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("unsupported remote mutation operation %q", operation),
			nil,
		)
	}
	if err != nil {
		return resource.Resource{}, err
	}

	payloadForLocal := resourceInfo.Payload
	if remotePayload != nil {
		payloadForLocal = remotePayload
	}

	maskedPayload, err := r.maskPayloadForLocal(ctx, payloadForLocal)
	if err != nil {
		return resource.Resource{}, err
	}

	if err := manager.Save(ctx, resourceInfo.LogicalPath, maskedPayload); err != nil {
		return resource.Resource{}, err
	}

	resourceInfo.Payload = maskedPayload
	return resourceInfo, nil
}

func (r *DefaultReconciler) resolvePayloadForRemote(ctx context.Context, value resource.Value) (resource.Value, error) {
	if value == nil {
		return nil, nil
	}

	if r == nil || r.SecretsProvider == nil {
		return resource.Normalize(value)
	}

	return r.SecretsProvider.ResolvePayload(ctx, value)
}

func (r *DefaultReconciler) maskPayloadForLocal(ctx context.Context, value resource.Value) (resource.Value, error) {
	if value == nil {
		return nil, nil
	}

	if r == nil || r.SecretsProvider == nil {
		return resource.Normalize(value)
	}

	return r.SecretsProvider.MaskPayload(ctx, value)
}

func (r *DefaultReconciler) fetchRemoteValue(ctx context.Context, resourceInfo resource.Resource) (resource.Value, error) {
	serverManager, err := r.serverManager()
	if err != nil {
		return nil, err
	}

	remoteValue, err := serverManager.Get(ctx, resourceInfo)
	if err == nil {
		return remoteValue, nil
	}
	if !isTypedCategory(err, faults.NotFoundError) {
		return nil, err
	}

	candidates, listErr := serverManager.List(ctx, resourceInfo.CollectionPath, resourceInfo.Metadata)
	if listErr != nil {
		return nil, listErr
	}

	matched := make([]resource.Resource, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.LocalAlias == resourceInfo.LocalAlias {
			matched = append(matched, candidate)
			continue
		}
		if resourceInfo.RemoteID != "" && candidate.RemoteID == resourceInfo.RemoteID {
			matched = append(matched, candidate)
		}
	}

	switch len(matched) {
	case 0:
		return nil, err
	case 1:
		return matched[0].Payload, nil
	default:
		return nil, faults.NewTypedError(
			faults.ConflictError,
			fmt.Sprintf("remote fallback for %q is ambiguous", resourceInfo.LogicalPath),
			nil,
		)
	}
}

func (r *DefaultReconciler) renderOperationSpec(
	ctx context.Context,
	resourceInfo resource.Resource,
	operation metadata.Operation,
	value resource.Value,
) (metadata.OperationSpec, error) {
	metadataCopy := cloneMetadata(resourceInfo.Metadata)
	if metadataCopy.Operations == nil {
		metadataCopy.Operations = map[string]metadata.OperationSpec{}
	}

	operationSpec := metadataCopy.Operations[string(operation)]
	if strings.TrimSpace(operationSpec.Path) == "" {
		if operation == metadata.OperationList {
			operationSpec.Path = resourceInfo.CollectionPath
		} else {
			operationSpec.Path = resourceInfo.LogicalPath
		}
		metadataCopy.Operations[string(operation)] = operationSpec
	}

	return metadata.ResolveOperationSpec(ctx, metadataCopy, operation, value)
}

func cloneMetadata(value metadata.ResourceMetadata) metadata.ResourceMetadata {
	cloned := metadata.ResourceMetadata{
		IDFromAttribute:    value.IDFromAttribute,
		AliasFromAttribute: value.AliasFromAttribute,
		Operations:         make(map[string]metadata.OperationSpec, len(value.Operations)),
		Filter:             cloneStringSlice(value.Filter),
		Suppress:           cloneStringSlice(value.Suppress),
		JQ:                 value.JQ,
	}

	for key, operationSpec := range value.Operations {
		cloned.Operations[key] = metadata.OperationSpec{
			Method:      operationSpec.Method,
			Path:        operationSpec.Path,
			Query:       cloneStringMap(operationSpec.Query),
			Headers:     cloneStringMap(operationSpec.Headers),
			Accept:      operationSpec.Accept,
			ContentType: operationSpec.ContentType,
			Body:        operationSpec.Body,
			Filter:      cloneStringSlice(operationSpec.Filter),
			Suppress:    cloneStringSlice(operationSpec.Suppress),
			JQ:          operationSpec.JQ,
		}
	}

	return cloned
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func applyCompareTransforms(value resource.Value, operationSpec metadata.OperationSpec) (resource.Value, error) {
	normalized, err := resource.Normalize(value)
	if err != nil {
		return nil, err
	}

	filtered := normalized
	if len(operationSpec.Filter) > 0 {
		filtered, err = applyFilterPointers(filtered, operationSpec.Filter)
		if err != nil {
			return nil, err
		}
	}

	if len(operationSpec.Suppress) == 0 {
		return filtered, nil
	}

	return applySuppressPointers(filtered, operationSpec.Suppress)
}

func applyFilterPointers(value resource.Value, pointers []string) (resource.Value, error) {
	normalizedPointers, err := normalizePointers(pointers)
	if err != nil {
		return nil, err
	}

	result := any(nil)
	for _, pointer := range normalizedPointers {
		tokens, err := parsePointerTokens(pointer)
		if err != nil {
			return nil, err
		}
		if len(tokens) == 0 {
			return deepCopyValue(value), nil
		}

		foundValue, found, err := lookupPointerValue(value, tokens)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}

		result, err = setPointerValue(result, tokens, foundValue)
		if err != nil {
			return nil, err
		}
	}

	if result == nil {
		switch value.(type) {
		case []any:
			return []any{}, nil
		case map[string]any:
			return map[string]any{}, nil
		default:
			return nil, nil
		}
	}

	return result, nil
}

func applySuppressPointers(value resource.Value, pointers []string) (resource.Value, error) {
	normalizedPointers, err := normalizePointers(pointers)
	if err != nil {
		return nil, err
	}

	working := deepCopyValue(value)
	for _, pointer := range normalizedPointers {
		tokens, err := parsePointerTokens(pointer)
		if err != nil {
			return nil, err
		}
		if len(tokens) == 0 {
			return nil, nil
		}

		working, err = deletePointerValue(working, tokens)
		if err != nil {
			return nil, err
		}
	}

	return working, nil
}

func normalizePointers(pointers []string) ([]string, error) {
	if len(pointers) == 0 {
		return nil, nil
	}

	normalized := make([]string, 0, len(pointers))
	seen := make(map[string]struct{}, len(pointers))

	for _, pointer := range pointers {
		value := strings.TrimSpace(pointer)
		if value == "" {
			value = "/"
		}
		if _, found := seen[value]; found {
			continue
		}
		if value != "/" && !strings.HasPrefix(value, "/") {
			return nil, faults.NewTypedError(
				faults.ValidationError,
				fmt.Sprintf("invalid compare pointer %q", pointer),
				nil,
			)
		}

		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}

	sort.Strings(normalized)
	return normalized, nil
}

func parsePointerTokens(pointer string) ([]string, error) {
	trimmed := strings.TrimSpace(pointer)
	if trimmed == "" || trimmed == "/" {
		return nil, nil
	}
	if !strings.HasPrefix(trimmed, "/") {
		return nil, faults.NewTypedError(
			faults.ValidationError,
			fmt.Sprintf("invalid compare pointer %q", pointer),
			nil,
		)
	}

	rawTokens := strings.Split(trimmed[1:], "/")
	tokens := make([]string, len(rawTokens))
	for idx, token := range rawTokens {
		unescaped := strings.ReplaceAll(token, "~1", "/")
		unescaped = strings.ReplaceAll(unescaped, "~0", "~")
		tokens[idx] = unescaped
	}
	return tokens, nil
}

func lookupPointerValue(value any, tokens []string) (any, bool, error) {
	current := value
	for _, token := range tokens {
		switch typed := current.(type) {
		case map[string]any:
			item, found := typed[token]
			if !found {
				return nil, false, nil
			}
			current = item
		case []any:
			index, ok := parseArrayIndex(token)
			if !ok || index < 0 || index >= len(typed) {
				return nil, false, nil
			}
			current = typed[index]
		default:
			return nil, false, nil
		}
	}

	return deepCopyValue(current), true, nil
}

func setPointerValue(root any, tokens []string, value any) (any, error) {
	if len(tokens) == 0 {
		return deepCopyValue(value), nil
	}

	head := tokens[0]
	tail := tokens[1:]

	if index, isIndex := parseArrayIndex(head); isIndex {
		var items []any
		switch typed := root.(type) {
		case nil:
			items = make([]any, index+1)
		case []any:
			items = typed
			if len(items) <= index {
				grown := make([]any, index+1)
				copy(grown, items)
				items = grown
			}
		default:
			return nil, faults.NewTypedError(
				faults.ValidationError,
				"compare pointer expects array segment",
				nil,
			)
		}

		next, err := setPointerValue(items[index], tail, value)
		if err != nil {
			return nil, err
		}
		items[index] = next
		return items, nil
	}

	var fields map[string]any
	switch typed := root.(type) {
	case nil:
		fields = map[string]any{}
	case map[string]any:
		fields = typed
	default:
		return nil, faults.NewTypedError(
			faults.ValidationError,
			"compare pointer expects object segment",
			nil,
		)
	}

	next, err := setPointerValue(fields[head], tail, value)
	if err != nil {
		return nil, err
	}
	fields[head] = next
	return fields, nil
}

func deletePointerValue(root any, tokens []string) (any, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	head := tokens[0]
	tail := tokens[1:]

	if index, isIndex := parseArrayIndex(head); isIndex {
		items, ok := root.([]any)
		if !ok {
			return root, nil
		}
		if index < 0 || index >= len(items) {
			return root, nil
		}

		if len(tail) == 0 {
			return append(items[:index], items[index+1:]...), nil
		}

		next, err := deletePointerValue(items[index], tail)
		if err != nil {
			return nil, err
		}
		items[index] = next
		return items, nil
	}

	fields, ok := root.(map[string]any)
	if !ok {
		return root, nil
	}

	if len(tail) == 0 {
		delete(fields, head)
		return fields, nil
	}

	child, found := fields[head]
	if !found {
		return root, nil
	}

	next, err := deletePointerValue(child, tail)
	if err != nil {
		return nil, err
	}
	if next == nil {
		delete(fields, head)
		return fields, nil
	}

	fields[head] = next
	return fields, nil
}

func parseArrayIndex(value string) (int, bool) {
	if value == "" {
		return 0, false
	}

	for _, char := range value {
		if char < '0' || char > '9' {
			return 0, false
		}
	}

	index, err := strconv.Atoi(value)
	if err != nil {
		return 0, false
	}
	return index, true
}

func deepCopyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		copied := make(map[string]any, len(typed))
		for key, item := range typed {
			copied[key] = deepCopyValue(item)
		}
		return copied
	case []any:
		copied := make([]any, len(typed))
		for idx := range typed {
			copied[idx] = deepCopyValue(typed[idx])
		}
		return copied
	default:
		return typed
	}
}

func buildDiffEntries(logicalPath string, local resource.Value, remote resource.Value) []resource.DiffEntry {
	entries := make([]resource.DiffEntry, 0)
	collectDiffEntries(&entries, logicalPath, "", local, remote)
	return entries
}

func collectDiffEntries(entries *[]resource.DiffEntry, logicalPath string, pointer string, local any, remote any) {
	if reflect.DeepEqual(local, remote) {
		return
	}

	localObject, localIsObject := local.(map[string]any)
	remoteObject, remoteIsObject := remote.(map[string]any)
	if localIsObject && remoteIsObject {
		keys := make([]string, 0, len(localObject)+len(remoteObject))
		seen := make(map[string]struct{}, len(localObject)+len(remoteObject))
		for key := range localObject {
			seen[key] = struct{}{}
			keys = append(keys, key)
		}
		for key := range remoteObject {
			if _, found := seen[key]; found {
				continue
			}
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			nextPointer := pointer + "/" + escapePointerToken(key)
			localValue, localFound := localObject[key]
			remoteValue, remoteFound := remoteObject[key]

			switch {
			case !localFound:
				appendDiffEntry(entries, logicalPath, nextPointer, "add", nil, remoteValue)
			case !remoteFound:
				appendDiffEntry(entries, logicalPath, nextPointer, "remove", localValue, nil)
			default:
				collectDiffEntries(entries, logicalPath, nextPointer, localValue, remoteValue)
			}
		}
		return
	}

	localArray, localIsArray := local.([]any)
	remoteArray, remoteIsArray := remote.([]any)
	if localIsArray && remoteIsArray {
		maxLength := len(localArray)
		if len(remoteArray) > maxLength {
			maxLength = len(remoteArray)
		}

		for idx := range maxLength {
			nextPointer := pointer + "/" + strconv.Itoa(idx)

			switch {
			case idx >= len(localArray):
				appendDiffEntry(entries, logicalPath, nextPointer, "add", nil, remoteArray[idx])
			case idx >= len(remoteArray):
				appendDiffEntry(entries, logicalPath, nextPointer, "remove", localArray[idx], nil)
			default:
				collectDiffEntries(entries, logicalPath, nextPointer, localArray[idx], remoteArray[idx])
			}
		}
		return
	}

	appendDiffEntry(entries, logicalPath, pointer, "replace", local, remote)
}

func appendDiffEntry(
	entries *[]resource.DiffEntry,
	logicalPath string,
	pointer string,
	operation string,
	local any,
	remote any,
) {
	*entries = append(*entries, resource.DiffEntry{
		Path:      buildDiffPath(logicalPath, pointer),
		Operation: operation,
		Local:     local,
		Remote:    remote,
	})
}

func buildDiffPath(logicalPath string, pointer string) string {
	if pointer == "" {
		return logicalPath
	}
	if logicalPath == "/" {
		return pointer
	}
	return logicalPath + pointer
}

func escapePointerToken(value string) string {
	escaped := strings.ReplaceAll(value, "~", "~0")
	return strings.ReplaceAll(escaped, "/", "~1")
}

func isDirectChildPath(parentPath string, logicalPath string) bool {
	if parentPath == "/" {
		return len(splitLogicalPathSegments(logicalPath)) == 1
	}

	parentSegments := splitLogicalPathSegments(parentPath)
	childSegments := splitLogicalPathSegments(logicalPath)
	if len(childSegments) != len(parentSegments)+1 {
		return false
	}

	for idx := range parentSegments {
		if parentSegments[idx] != childSegments[idx] {
			return false
		}
	}
	return true
}

func splitLogicalPathSegments(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

func isTypedCategory(err error, category faults.ErrorCategory) bool {
	typedErr, ok := err.(*faults.TypedError)
	return ok && typedErr.Category == category
}
