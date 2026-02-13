package reconciler

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"
)

func (r *DefaultReconciler) InitRepositoryLocal() error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	if initializer, ok := r.ResourceRepositoryManager.(repository.LocalRepositoryInitializer); ok {
		return initializer.InitLocalRepository()
	}
	return r.ResourceRepositoryManager.Init()
}

func (r *DefaultReconciler) InitRepositoryRemoteIfEmpty() (bool, error) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return false, errors.New("resource repository manager is not configured")
	}
	if initializer, ok := r.ResourceRepositoryManager.(repository.RemoteRepositoryInitializer); ok {
		return initializer.InitRemoteIfEmpty()
	}
	return false, nil
}

func (r *DefaultReconciler) RefreshRepository() error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	rebaser, ok := r.ResourceRepositoryManager.(repository.ResourceRepositoryRebaser)
	if !ok {
		return errors.New("repository refresh is not supported by the configured repository")
	}
	return rebaser.RebaseLocalFromRemote()
}

func (r *DefaultReconciler) UpdateRemoteRepositoryWithForce(force bool) error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	if !force {
		pusher, ok := r.ResourceRepositoryManager.(repository.ResourceRepositoryPusher)
		if !ok {
			return errors.New("repository push is not supported by the configured repository")
		}
		return pusher.PushLocalDiffsToRemote()
	}
	pusher, ok := r.ResourceRepositoryManager.(repository.ResourceRepositoryForcePusher)
	if !ok {
		return errors.New("force push is not supported by the configured repository")
	}
	return pusher.ForcePushLocalDiffsToRemote()
}

func (r *DefaultReconciler) ResetRepository() error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	resetter, ok := r.ResourceRepositoryManager.(repository.ResourceRepositoryResetter)
	if !ok {
		return errors.New("repository reset is not supported by the configured repository")
	}
	return resetter.ResetLocal()
}

func (r *DefaultReconciler) RepositoryPathsInCollection(path string) ([]string, error) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return nil, errors.New("resource repository manager is not configured")
	}
	collectionPath, err := r.normalizeCollectionPath(path)
	if err != nil {
		return nil, err
	}
	base := strings.TrimRight(collectionPath, "/")
	prefix := base
	if prefix != "/" {
		prefix += "/"
	}

	var results []string
	for _, entry := range r.ResourceRepositoryManager.ListResourcePaths() {
		if entry == base || strings.HasPrefix(entry, prefix) {
			results = append(results, entry)
		}
	}
	return results, nil
}

func (r *DefaultReconciler) runRepositoryBatch(fn func() error) error {
	if fn == nil {
		return errors.New("batch function is required")
	}
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	if batcher, ok := r.ResourceRepositoryManager.(repository.ResourceRepositoryBatcher); ok {
		return batcher.RunBatch(fn)
	}
	return fn()
}

func (r *DefaultReconciler) ResourceMetadata(path string) (resource.ResourceMetadata, error) {
	if r == nil || r.ResourceRecordProvider == nil {
		return resource.ResourceMetadata{}, errors.New("resource record provider is not configured")
	}
	if err := r.validateMetadataPath(path); err != nil {
		return resource.ResourceMetadata{}, err
	}
	record, err := r.ResourceRecordProvider.GetResourceRecord(path)
	if err != nil {
		return resource.ResourceMetadata{}, err
	}
	return record.Meta, nil
}

func (r *DefaultReconciler) SecretPathsFor(path string) ([]string, error) {
	if r == nil || r.ResourceRecordProvider == nil {
		return nil, nil
	}
	if err := r.validateLogicalPath(path); err != nil {
		return nil, err
	}
	meta, err := r.ResourceRecordProvider.GetMergedMetadata(path)
	if err != nil {
		return nil, err
	}
	if meta.ResourceInfo == nil || len(meta.ResourceInfo.SecretInAttributes) == 0 {
		return nil, nil
	}
	return meta.ResourceInfo.SecretInAttributes, nil
}

func (r *DefaultReconciler) MaskResourceSecrets(path string, res resource.Resource, store bool) (resource.Resource, error) {
	paths, err := r.SecretPathsFor(path)
	if err != nil {
		return resource.Resource{}, err
	}
	return secrets.MaskResourceSecrets(res, path, paths, r.SecretsManager, store)
}

func (r *DefaultReconciler) ResolveResourceSecrets(path string, res resource.Resource) (resource.Resource, error) {
	paths, err := r.SecretPathsFor(path)
	if err != nil {
		return resource.Resource{}, err
	}
	return secrets.ResolveResourceSecrets(res, path, paths, r.SecretsManager)
}

func (r *DefaultReconciler) SaveLocalResourceWithSecrets(path string, res resource.Resource, storeSecrets bool) error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	if r.ResourceRecordProvider == nil {
		return errors.New("resource record provider is not configured")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return err
	}

	record, err := r.ResourceRecordProvider.GetResourceRecord(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(record.Path) == "" {
		record.Path = path
	}
	payload := record.ReadPayload()
	processed, err := record.ApplyPayload(res, payload)
	if err != nil {
		return err
	}

	targetPath := record.AliasPath(processed)
	if err := r.validateLogicalPath(targetPath); err != nil {
		return err
	}

	if storeSecrets {
		processed, err = r.MaskResourceSecrets(targetPath, processed, true)
		if err != nil {
			return err
		}
	}

	return r.ResourceRepositoryManager.ApplyResource(targetPath, processed)
}

func (r *DefaultReconciler) SaveLocalCollectionItemsWithSecrets(path string, items []resource.Resource, storeSecrets bool) error {
	if r == nil || r.ResourceRepositoryManager == nil {
		return errors.New("resource repository manager is not configured")
	}
	if r.ResourceRecordProvider == nil {
		return errors.New("resource record provider is not configured")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return err
	}

	record, err := r.ResourceRecordProvider.GetResourceRecord(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(record.Path) == "" {
		record.Path = path
	}

	basePath := strings.TrimRight(resource.NormalizePath(path), "/")
	return r.runRepositoryBatch(func() error {
		for idx, item := range items {
			aliasPath := record.AliasPath(item)
			alias := resource.LastSegment(aliasPath)
			if alias == "" || resource.NormalizePath(aliasPath) == resource.NormalizePath(record.Path) {
				alias = resource.LastSegment(record.RemoteResourcePath(item))
			}
			if alias == "" {
				alias = fmt.Sprintf("%d", idx)
			}

			targetPath := resource.NormalizePath(basePath + "/" + alias)
			if err := r.validateLogicalPath(targetPath); err != nil {
				return err
			}

			targetRecord, err := r.ResourceRecordProvider.GetResourceRecord(targetPath)
			if err != nil {
				return err
			}
			if strings.TrimSpace(targetRecord.Path) == "" {
				targetRecord.Path = targetPath
			}
			payload := targetRecord.ReadPayload()
			processed, err := targetRecord.ApplyPayload(item, payload)
			if err != nil {
				return err
			}

			if storeSecrets {
				processed, err = r.MaskResourceSecrets(targetPath, processed, true)
				if err != nil {
					return err
				}
			}

			if err := r.ResourceRepositoryManager.ApplyResource(targetPath, processed); err != nil {
				return err
			}
		}
		return nil
	})
}

type RemoteResourceEntry struct {
	Path      string
	ID        string
	Alias     string
	AliasPath string
}

func (r *DefaultReconciler) ListRemoteResourceEntries(path string) ([]RemoteResourceEntry, error) {
	end := r.beginRemoteOperation()
	defer end()
	if r == nil || r.ResourceServerManager == nil {
		return nil, errors.New("resource server manager is not configured")
	}
	collectionLogicalPath, err := r.normalizeCollectionPath(path)
	if err != nil {
		return nil, err
	}
	collectionPath, record, replacements, err := r.resolveRemoteCollectionPath(collectionLogicalPath)
	if err != nil {
		return nil, err
	}

	items, err := r.fetchCollection(record, replacePathSegments(collectionPath, replacements), false)
	if err != nil {
		return nil, err
	}

	var entries []RemoteResourceEntry
	for _, item := range items {
		recordPath := resource.NormalizePath(record.Path)
		idValue := attributeValueFromResource(item, record.Meta.ResourceInfo, true, "")
		aliasValue := attributeValueFromResource(item, record.Meta.ResourceInfo, false, "")
		segment := idValue
		if segment == "" {
			segment = aliasValue
		}
		if segment == "" {
			segment = resource.LastSegment(recordPath)
		}
		remotePath := collectionResourcePath(record.CollectionPath(), segment)
		if remotePath == "" {
			continue
		}
		aliasPath := record.AliasPath(item)
		alias := resource.LastSegment(aliasPath)
		normAliasPath := resource.NormalizePath(aliasPath)

		if alias == "" || normAliasPath == recordPath {
			alias = resource.LastSegment(remotePath)
			normAliasPath = remotePath
		}
		if alias == "" {
			alias = resource.LastSegment(recordPath)
			if normAliasPath == "" {
				normAliasPath = recordPath
			}
		}
		if normAliasPath == "" {
			normAliasPath = remotePath
		}
		if idValue == "" {
			idValue = resource.LastSegment(remotePath)
		}
		if aliasValue == "" {
			aliasValue = alias
		}
		entries = append(entries, RemoteResourceEntry{
			Path:      remotePath,
			ID:        idValue,
			Alias:     aliasValue,
			AliasPath: normAliasPath,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		a := entries[i].AliasPath
		if a == "" {
			a = entries[i].Path
		}
		b := entries[j].AliasPath
		if b == "" {
			b = entries[j].Path
		}
		return a < b
	})
	return entries, nil
}

func attributeValueFromResource(item resource.Resource, info *resource.ResourceInfoMetadata, idAttr bool, fallback string) string {
	if info != nil {
		var attr string
		if idAttr {
			attr = strings.TrimSpace(info.IDFromAttribute)
		} else {
			attr = strings.TrimSpace(info.AliasFromAttribute)
		}
		if attr != "" {
			if value, ok := resource.LookupValueFromResource(item, attr); ok {
				value = strings.TrimSpace(value)
				if value != "" {
					return value
				}
			}
		}
	}
	return strings.TrimSpace(fallback)
}

func collectionResourcePath(collectionPath, segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return ""
	}
	base := strings.TrimRight(strings.TrimSpace(collectionPath), "/")
	if base == "" {
		base = "/"
	}
	segment = sanitizeResourceSegment(segment)
	if base == "/" {
		return resource.NormalizePath("/" + segment)
	}
	return resource.NormalizePath(base + "/" + segment)
}

func sanitizeResourceSegment(segment string) string {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return ""
	}
	segment = strings.ReplaceAll(segment, "/", "-")
	segment = strings.ReplaceAll(segment, "\\", "-")
	return segment
}

func (r *DefaultReconciler) ListRemoteResourcePaths(path string) ([]string, error) {
	entries, err := r.ListRemoteResourceEntries(path)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	return paths, nil
}

func (r *DefaultReconciler) ListRemoteResourcePathsFromLocal() ([]string, error) {
	if r == nil || r.ResourceRepositoryManager == nil {
		return nil, errors.New("resource repository manager is not configured")
	}

	localPaths := r.ResourceRepositoryManager.ListResourcePaths()
	if len(localPaths) == 0 {
		return nil, nil
	}

	collections := make(map[string]struct{})
	for _, path := range localPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		collections[collectionPathFromResource(path)] = struct{}{}
	}

	var collectionPaths []string
	for path := range collections {
		collectionPaths = append(collectionPaths, path)
	}
	sort.Strings(collectionPaths)

	var results []string
	seen := make(map[string]struct{})
	for _, collection := range collectionPaths {
		paths, err := r.ListRemoteResourcePaths(collection)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			if _, ok := seen[path]; ok {
				continue
			}
			seen[path] = struct{}{}
			results = append(results, path)
		}
	}

	sort.Strings(results)
	return results, nil
}

func (r *DefaultReconciler) normalizeCollectionPath(path string) (string, error) {
	if err := r.validateLogicalPath(path); err != nil {
		return "", err
	}
	return r.forceCollectionPath(path), nil
}

func (r *DefaultReconciler) forceCollectionPath(path string) string {
	normalized := resource.NormalizePath(path)
	if normalized != "/" {
		return normalized + "/"
	}
	return normalized
}

func collectionPathFromResource(path string) string {
	normalized := resource.NormalizePath(path)
	if normalized == "/" {
		return normalized
	}
	last := resource.LastSegment(normalized)
	if last == "" {
		return "/"
	}
	base := strings.TrimSuffix(normalized, "/"+last)
	if strings.TrimSpace(base) == "" {
		return "/"
	}
	return resource.NormalizePath(base)
}

func (r *DefaultReconciler) InitSecrets() error {
	if r == nil || r.SecretsManager == nil {
		return secrets.ErrSecretStoreNotConfigured
	}
	return r.SecretsManager.Init()
}

func (r *DefaultReconciler) EnsureSecretsFile() error {
	if r == nil || r.SecretsManager == nil {
		return secrets.ErrSecretStoreNotConfigured
	}
	if initializer, ok := r.SecretsManager.(secrets.FileEnsurer); ok {
		return initializer.EnsureFile()
	}
	return nil
}

func (r *DefaultReconciler) GetSecret(resourcePath string, key string) (string, error) {
	if r == nil || r.SecretsManager == nil {
		return "", secrets.ErrSecretStoreNotConfigured
	}
	if err := r.validateLogicalPath(resourcePath); err != nil {
		return "", err
	}
	return r.SecretsManager.GetSecret(resourcePath, key)
}

func (r *DefaultReconciler) SetSecret(resourcePath string, key string, value string) error {
	if r == nil || r.SecretsManager == nil {
		return secrets.ErrSecretStoreNotConfigured
	}
	if err := r.validateLogicalPath(resourcePath); err != nil {
		return err
	}
	return r.SecretsManager.SetSecret(resourcePath, key, value)
}

func (r *DefaultReconciler) DeleteSecret(resourcePath string, key string) error {
	if r == nil || r.SecretsManager == nil {
		return secrets.ErrSecretStoreNotConfigured
	}
	if err := r.validateLogicalPath(resourcePath); err != nil {
		return err
	}
	return r.SecretsManager.DeleteSecret(resourcePath, key)
}

func (r *DefaultReconciler) ListSecretKeys(resourcePath string) ([]string, error) {
	if r == nil || r.SecretsManager == nil {
		return nil, secrets.ErrSecretStoreNotConfigured
	}
	if err := r.validateLogicalPath(resourcePath); err != nil {
		return nil, err
	}
	return r.SecretsManager.ListKeys(resourcePath)
}

func (r *DefaultReconciler) ListSecretResources() ([]string, error) {
	if r == nil || r.SecretsManager == nil {
		return nil, secrets.ErrSecretStoreNotConfigured
	}
	return r.SecretsManager.ListResources()
}
