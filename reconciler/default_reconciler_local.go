package reconciler

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

type LocalResourceUpdateResult struct {
	OriginalPath string
	UpdatedPath  string
	Moved        bool
}

func (r *DefaultReconciler) GetLocalResource(path string) (resource.Resource, error) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return resource.Resource{}, errors.New("resource repository manager is not configured")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return resource.Resource{}, err
	}
	return r.ResourceRepositoryManager.GetResource(path)
}

func (r *DefaultReconciler) DeleteLocalResource(path string) error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return err
	}
	return r.ResourceRepositoryManager.DeleteResource(path)
}

func (r *DefaultReconciler) SaveLocalResource(path string, data resource.Resource) error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return err
	}
	record, err := r.recordFor(path)
	if err != nil {
		return err
	}
	payload := record.ReadPayload()
	processed, err := record.ApplyPayload(data, payload)
	if err != nil {
		return err
	}
	targetPath := record.AliasPath(processed)
	if err := r.validateLogicalPath(targetPath); err != nil {
		return err
	}
	return r.ResourceRepositoryManager.ApplyResource(targetPath, processed)
}

func (r *DefaultReconciler) SaveLocalCollectionItems(path string, items []resource.Resource) error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return err
	}
	record, err := r.recordFor(path)
	if err != nil {
		return err
	}

	basePath := resource.NormalizePath(path)
	basePath = strings.TrimRight(basePath, "/")

	for idx, item := range items {
		alias := resource.LastSegment(record.AliasPath(item))
		if alias == "" {
			alias = resource.LastSegment(record.RemoteResourcePath(item))
		}
		if alias == "" {
			alias = fmt.Sprintf("%d", idx)
		}
		targetPath := resource.NormalizePath(basePath + "/" + alias)
		if err := r.validateLogicalPath(targetPath); err != nil {
			return err
		}

		targetRecord, err := r.recordFor(targetPath)
		if err != nil {
			return err
		}
		payload := targetRecord.ReadPayload()
		processed, err := targetRecord.ApplyPayload(item, payload)
		if err != nil {
			return err
		}
		if err := r.ResourceRepositoryManager.ApplyResource(targetPath, processed); err != nil {
			return err
		}
	}
	return nil
}

func (r *DefaultReconciler) UpdateLocalResourcesForMetadata(path string) ([]LocalResourceUpdateResult, error) {
	if r == nil {
		return nil, errors.New("reconciler is nil")
	}
	if r.ResourceRepositoryManager == nil {
		return nil, errors.New("resource repository manager is not configured")
	}
	if r.ResourceRecordProvider == nil {
		return nil, errors.New("resource record provider is not configured")
	}
	if err := r.validateMetadataPath(path); err != nil {
		return nil, err
	}

	var targets []string
	if resource.IsCollectionPath(path) {
		paths, err := r.localChildResources(path)
		if err != nil {
			return nil, err
		}
		targets = paths
	} else {
		targets = []string{resource.NormalizePath(path)}
	}

	var results []LocalResourceUpdateResult
	for _, target := range targets {
		res, err := r.GetLocalResource(target)
		if err != nil {
			return nil, fmt.Errorf("update resource %s: %w", target, err)
		}

		updatedPath, moved, err := r.updateLocalResourceForMetadata(target, res)
		if err != nil {
			return nil, fmt.Errorf("update resource %s: %w", target, err)
		}
		results = append(results, LocalResourceUpdateResult{
			OriginalPath: target,
			UpdatedPath:  updatedPath,
			Moved:        moved,
		})
	}

	return results, nil
}

func (r *DefaultReconciler) UpdateLocalMetadata(path string, update func(map[string]any) (bool, error)) error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	if err := r.validateMetadataPath(path); err != nil {
		return err
	}
	if update == nil {
		return errors.New("metadata update function is required")
	}

	manager, ok := r.ResourceRepositoryManager.(repository.MetadataRepositoryManager)
	if !ok {
		return errors.New("resource repository manager does not support metadata operations")
	}

	meta, err := manager.ReadMetadata(path)
	if err != nil {
		return err
	}
	if meta == nil {
		meta = map[string]any{}
	}

	changed, err := update(meta)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	if len(meta) == 0 {
		return manager.DeleteMetadata(path)
	}
	return manager.WriteMetadata(path, meta)
}

func (r *DefaultReconciler) WriteLocalMetadata(path string, metadata map[string]any) error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	if err := r.validateMetadataPath(path); err != nil {
		return err
	}

	manager, ok := r.ResourceRepositoryManager.(repository.MetadataRepositoryManager)
	if !ok {
		return errors.New("resource repository manager does not support metadata operations")
	}
	if metadata == nil {
		metadata = map[string]any{}
	}
	return manager.WriteMetadata(path, metadata)
}

func (r *DefaultReconciler) localChildResources(path string) ([]string, error) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return nil, errors.New("resource repository manager is not configured")
	}

	base := strings.TrimRight(resource.NormalizePath(path), "/")
	pattern := resource.SplitPathSegments(base)
	baseDepth := len(pattern)

	var results []string
	for _, entry := range r.ResourceRepositoryManager.ListResourcePaths() {
		normalized := resource.NormalizePath(entry)
		if normalized == base {
			continue
		}
		if pathDepth(normalized) == baseDepth+1 {
			if pathMatchesPattern(normalized, pattern) {
				results = append(results, normalized)
			}
		}
	}
	sort.Strings(results)
	return results, nil
}

func pathMatchesPattern(path string, pattern []string) bool {
	segments := resource.SplitPathSegments(path)
	if len(segments) < len(pattern) {
		return false
	}
	for idx, part := range pattern {
		if part == "_" {
			continue
		}
		if segments[idx] != part {
			return false
		}
	}
	return true
}

func (r *DefaultReconciler) updateLocalResourceForMetadata(path string, res resource.Resource) (string, bool, error) {
	record, err := r.ResourceRecordProvider.GetResourceRecord(path)
	if err != nil {
		return "", false, err
	}
	if strings.TrimSpace(record.Path) == "" {
		record.Path = path
	}

	payload := record.ReadPayload()
	processed, err := record.ApplyPayload(res, payload)
	if err != nil {
		return "", false, err
	}

	targetPath := record.AliasPath(processed)
	if strings.TrimSpace(targetPath) == "" {
		targetPath = path
	}
	targetPath = resource.NormalizePath(targetPath)
	if err := r.validateLogicalPath(targetPath); err != nil {
		return "", false, err
	}

	normalizedPath := resource.NormalizePath(path)
	moved := false
	if targetPath != normalizedPath {
		mover, ok := r.ResourceRepositoryManager.(repository.ResourceRepositoryMover)
		if !ok {
			return "", false, errors.New("resource repository manager does not support moving resources")
		}
		if err := mover.MoveResourceTree(normalizedPath, targetPath); err != nil {
			return "", false, err
		}
		moved = true
	}

	if err := r.ResourceRepositoryManager.ApplyResource(targetPath, processed); err != nil {
		return "", false, err
	}

	return targetPath, moved, nil
}

func pathDepth(path string) int {
	trimmed := strings.Trim(path, "/")
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "/"))
}

func (r *DefaultReconciler) ListLocalResourcePaths() []string {
	if r == nil || r.ResourceRepositoryManager == nil {
		return []string{}
	}
	if lister, ok := r.ResourceRepositoryManager.(repository.ResourceRepositoryPathLister); ok {
		paths, err := lister.ListResourcePathsWithErrors()
		if err == nil {
			return paths
		}
	}
	return r.ResourceRepositoryManager.ListResourcePaths()
}

func (r *DefaultReconciler) DiffResource(path string) (resource.ResourcePatch, error) {
	if r == nil {
		return resource.ResourcePatch{}, errors.New("reconciler is nil")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return resource.ResourcePatch{}, err
	}

	record, err := r.recordFor(path)
	if err != nil {
		return resource.ResourcePatch{}, err
	}

	local, err := r.GetLocalResource(path)
	if err != nil {
		return resource.ResourcePatch{}, err
	}

	remote, err := r.GetRemoteResource(path)
	if err != nil {
		return resource.ResourcePatch{}, err
	}

	if record.Meta.ResourceInfo != nil && len(record.Meta.ResourceInfo.SecretInAttributes) > 0 {
		secretPaths := record.Meta.ResourceInfo.SecretInAttributes
		placeholders, err := secrets.SecretPlaceholders(local, secretPaths)
		if err != nil {
			return resource.ResourcePatch{}, err
		}
		local, err = secrets.NormalizeResourceSecrets(local, secretPaths, placeholders)
		if err != nil {
			return resource.ResourcePatch{}, err
		}
		remote, err = secrets.NormalizeResourceSecrets(remote, secretPaths, placeholders)
		if err != nil {
			return resource.ResourcePatch{}, err
		}
	}

	var compareRules *resource.CompareMetadata
	if record.Meta.OperationInfo != nil {
		compareRules = record.Meta.OperationInfo.CompareResources
	}

	localPrepared, err := resource.ApplyCompareRules(local, compareRules)
	if err != nil {
		return resource.ResourcePatch{}, err
	}
	remotePrepared, err := resource.ApplyCompareRules(remote, compareRules)
	if err != nil {
		return resource.ResourcePatch{}, err
	}

	if reflect.DeepEqual(localPrepared.V, remotePrepared.V) {
		return resource.ResourcePatch{}, nil
	}

	patch := resource.BuildJSONPatch(localPrepared.V, remotePrepared.V)
	return patch, nil
}

func (r *DefaultReconciler) GetLocalResourcePath(path string) (string, error) {
	if err := r.validateLogicalPath(path); err != nil {
		return path, err
	}
	record, err := r.recordFor(path)
	if err != nil {
		return path, err
	}

	if record.Meta.ResourceInfo == nil || strings.TrimSpace(record.Meta.ResourceInfo.AliasFromAttribute) == "" {
		return path, nil
	}

	res, err := r.GetLocalResource(path)
	if err != nil {
		return path, err
	}

	return record.AliasPath(res), nil
}

func (r *DefaultReconciler) GetLocalCollectionPath(path string) (string, error) {
	if err := r.validateLogicalPath(path); err != nil {
		return path, err
	}
	normalized := resource.NormalizePath(path)
	if resource.IsCollectionPath(path) {
		return normalized, nil
	}
	last := resource.LastSegment(normalized)
	if last == "" {
		return normalized, nil
	}
	return resource.NormalizePath(strings.TrimSuffix(normalized, "/"+last)), nil
}
