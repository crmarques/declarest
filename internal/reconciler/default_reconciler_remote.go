package reconciler

import (
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"declarest/internal/managedserver"
	"declarest/internal/resource"
	"declarest/internal/secrets"
)

func (r *DefaultReconciler) GetRemoteResource(path string) (resource.Resource, error) {
	if r == nil || r.ResourceServerManager == nil {
		return resource.Resource{}, errors.New("resource server manager is not configured")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return resource.Resource{}, err
	}

	isCollection := resource.IsCollectionPath(path)
	if isCollection {
		targetPath, record, _, err := r.resolveRemoteCollectionPath(path)
		if err != nil {
			return resource.Resource{}, err
		}

		items, err := r.fetchCollection(record, targetPath, false)
		if err != nil {
			return resource.Resource{}, err
		}
		payload := record.ListPayload()
		processed := make([]any, 0, len(items))
		for _, item := range items {
			transformed, err := record.ApplyPayload(item, payload)
			if err != nil {
				return resource.Resource{}, err
			}
			processed = append(processed, transformed.V)
		}
		return resource.NewResource(processed)
	}

	record, err := r.recordFor(path)
	if err != nil {
		return resource.Resource{}, err
	}

	var data resource.Resource
	if local, err := r.GetLocalResource(path); err == nil {
		data = local
	}

	var (
		resolvedPath    string
		resolvedRecord  resource.ResourceRecord
		replacements    map[string]string
		resolutionReady bool
	)
	resolve := func() error {
		if resolutionReady {
			return nil
		}
		var err error
		resolvedPath, resolvedRecord, replacements, err = r.resolveRemoteResourcePath(path, data)
		if err == nil {
			resolutionReady = true
		}
		return err
	}

	literalOp := record.ReadOperation(false)
	literalSpec, err := r.buildRequestSpecWithTarget(record, path, "", literalOp, false)
	if err != nil {
		return resource.Resource{}, err
	}
	if fetched, err := r.ResourceServerManager.GetResource(literalSpec); err == nil {
		payload := record.ReadPayload()
		return record.ApplyPayload(fetched, payload)
	} else if managedserver.IsNotFoundError(err) {
		if err := resolve(); err != nil {
			return resource.Resource{}, err
		}

		if data.V == nil {
			if target := resource.LastSegment(path); target != "" {
				if fromCollection, ok, lookupErr := r.findResourceInCollection(resolvedRecord, replacements, target); lookupErr == nil && ok {
					payload := resolvedRecord.ReadPayload()
					return resolvedRecord.ApplyPayload(fromCollection, payload)
				} else if lookupErr != nil {
					return resource.Resource{}, lookupErr
				}
			}
		}
	} else {
		return resource.Resource{}, err
	}

	if err := resolve(); err != nil {
		return resource.Resource{}, err
	}
	targetPath := resolvedPath

	if exists, resolved, resolvedData, err := r.remoteResourceExists(targetPath, resolvedRecord, data, replacements); err == nil && exists {
		targetPath = resolved
		if resolvedData.V != nil {
			resolvedRecord.Data = resolvedData
		}
	} else if err != nil {
		return resource.Resource{}, err
	}

	op := adjustOperation(resolvedRecord.ReadOperation(false))
	spec, err := r.buildRequestSpecWithTarget(resolvedRecord, targetPath, path, op, false)
	if err != nil {
		return resource.Resource{}, err
	}

	fetched, err := r.ResourceServerManager.GetResource(spec)
	if err != nil {
		return resource.Resource{}, err
	}

	payload := resolvedRecord.ReadPayload()
	return resolvedRecord.ApplyPayload(fetched, payload)
}

func (r *DefaultReconciler) DeleteRemoteResource(path string) error {
	if r == nil || r.ResourceServerManager == nil {
		return errors.New("resource server manager is not configured")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return err
	}

	var (
		record       resource.ResourceRecord
		replacements map[string]string
		remotePath   string
		err          error
	)

	var data resource.Resource
	if res, err := r.GetLocalResource(path); err == nil {
		data = res
	}

	remotePath, record, replacements, err = r.resolveRemoteResourcePath(path, data)
	if err != nil {
		return err
	}

	op := adjustOperation(record.DeleteOperation())
	spec, err := r.buildRequestSpecWithTarget(record, remotePath, path, op, false)
	if err != nil {
		return err
	}

	if err := r.ResourceServerManager.DeleteResource(spec); err != nil {
		if !managedserver.IsNotFoundError(err) {
			return err
		}

		altPath, altData, ok, lookupErr := r.findRemotePathByAlias(record, record.Data, replacements)
		if lookupErr != nil {
			return lookupErr
		}
		if !ok {

			return nil
		}
		if altData.V != nil {
			record.Data = altData
		}
		specAlt, specErr := r.buildRequestSpecWithTarget(record, altPath, path, op, false)
		if specErr != nil {
			return specErr
		}
		return r.ResourceServerManager.DeleteResource(specAlt)
	}

	return nil
}

func (r *DefaultReconciler) SaveRemoteResource(path string, data resource.Resource) error {
	if r == nil || r.ResourceServerManager == nil {
		return errors.New("resource server manager is not configured")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return err
	}

	remotePath, record, replacements, err := r.resolveRemoteResourcePath(path, data)
	if err != nil {
		return err
	}

	exists, resolvedPath, resolvedData, err := r.remoteResourceExists(remotePath, record, data, replacements)
	if err != nil {
		return err
	}

	if exists {
		if resolvedData.V != nil {
			record.Data = resolvedData
		}
		if err := r.updateRemoteResource(resolvedPath, record, data); err != nil {
			if managedserver.IsNotFoundError(err) {

				return r.createRemoteResource(record, data)
			}
			return err
		}
		return nil
	}

	if err := r.createRemoteResource(record, data); err != nil {
		if managedserver.IsConflictError(err) {
			if altPath, altData, ok, lookupErr := r.findRemotePathByAlias(record, data, replacements); lookupErr == nil && ok {
				if altData.V != nil {
					record.Data = altData
				}
				return r.updateRemoteResource(altPath, record, data)
			} else if lookupErr != nil {
				return lookupErr
			}
		}
		return err
	}

	return nil
}

func (r *DefaultReconciler) CreateRemoteResource(path string, data resource.Resource) error {
	if r == nil || r.ResourceServerManager == nil {
		return errors.New("resource server manager is not configured")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return err
	}

	_, record, _, err := r.resolveRemoteResourcePath(path, data)
	if err != nil {
		return err
	}

	return r.createRemoteResource(record, data)
}

func (r *DefaultReconciler) UpdateRemoteResource(path string, data resource.Resource) error {
	if r == nil || r.ResourceServerManager == nil {
		return errors.New("resource server manager is not configured")
	}
	if err := r.validateLogicalPath(path); err != nil {
		return err
	}

	remotePath, record, replacements, err := r.resolveRemoteResourcePath(path, data)
	if err != nil {
		return err
	}

	exists, resolvedPath, resolvedData, err := r.remoteResourceExists(remotePath, record, data, replacements)
	if err != nil {
		return err
	}
	if !exists {
		method := "PUT"
		if op := adjustOperation(record.UpdateOperation()); op != nil && strings.TrimSpace(op.HTTPMethod) != "" {
			method = strings.ToUpper(strings.TrimSpace(op.HTTPMethod))
		}
		return &managedserver.HTTPError{Method: method, URL: remotePath, StatusCode: http.StatusNotFound}
	}

	if resolvedData.V != nil {
		record.Data = resolvedData
	}
	return r.updateRemoteResource(resolvedPath, record, data)
}

func (r *DefaultReconciler) SyncRemoteResource(string) error {
	return errors.New("remote resource sync is not implemented")
}

func (r *DefaultReconciler) GetRemoteResourcePath(path string) (string, error) {
	if err := r.validateLogicalPath(path); err != nil {
		return path, err
	}
	var data resource.Resource
	if res, err := r.GetLocalResource(path); err == nil {
		data = res
	}
	remotePath, _, _, err := r.resolveRemoteResourcePath(path, data)
	if err != nil {
		return path, err
	}
	return remotePath, nil
}

func (r *DefaultReconciler) GetRemoteCollectionPath(path string) (string, error) {
	if err := r.validateLogicalPath(path); err != nil {
		return path, err
	}
	collectionPath, _, _, err := r.resolveRemoteCollectionPath(path)
	if err != nil {
		return path, err
	}
	return collectionPath, nil
}

func (r *DefaultReconciler) remoteResourceExists(path string, record resource.ResourceRecord, data resource.Resource, replacements map[string]string) (bool, string, resource.Resource, error) {
	if r.ResourceServerManager == nil {
		return false, path, resource.Resource{}, errors.New("resource server manager is not configured")
	}
	op := adjustOperation(record.ReadOperation(false))
	if op == nil {
		return false, path, resource.Resource{}, nil
	}
	spec, err := r.buildRequestSpecWithTarget(record, path, record.Path, op, false)
	if err != nil {
		return false, path, resource.Resource{}, err
	}
	exists, err := r.ResourceServerManager.ResourceExists(spec)
	if err != nil {
		return false, path, resource.Resource{}, err
	}
	if exists {
		return true, path, resource.Resource{}, nil
	}

	altPath, altData, ok, err := r.findRemotePathByAlias(record, data, replacements)
	if err != nil {
		return false, path, resource.Resource{}, err
	}
	if ok {
		return true, altPath, altData, nil
	}

	return false, path, resource.Resource{}, nil
}

func (r *DefaultReconciler) createRemoteResource(record resource.ResourceRecord, data resource.Resource) error {
	resolved, err := r.resolveSecretsForRemote(record, data)
	if err != nil {
		return err
	}
	data = resolved
	op := adjustOperation(record.CreateOperation())
	targetPath := record.CollectionPath()
	spec, err := r.buildRequestSpecWithTarget(record, targetPath, record.Path, op, true)
	if err != nil {
		return err
	}
	payload := record.OperationPayload(op)
	processed, err := record.ApplyPayload(data, payload)
	if err != nil {
		return err
	}
	return r.ResourceServerManager.CreateResource(processed, spec)
}

func (r *DefaultReconciler) updateRemoteResource(path string, record resource.ResourceRecord, data resource.Resource) error {
	resolved, err := r.resolveSecretsForRemote(record, data)
	if err != nil {
		return err
	}
	data = resolved
	op := adjustOperation(record.UpdateOperation())
	spec, err := r.buildRequestSpecWithTarget(record, path, record.Path, op, false)
	if err != nil {
		return err
	}
	payload := record.OperationPayload(op)
	processed, err := record.ApplyPayload(data, payload)
	if err != nil {
		return err
	}
	return r.ResourceServerManager.UpdateResource(processed, spec)
}

func (r *DefaultReconciler) resolveSecretsForRemote(record resource.ResourceRecord, data resource.Resource) (resource.Resource, error) {
	info := record.Meta.ResourceInfo
	if info == nil || len(info.SecretInAttributes) == 0 {
		return data, nil
	}
	if r.SecretsManager == nil {
		hasPlaceholders, err := secrets.HasSecretPlaceholders(data, info.SecretInAttributes)
		if err != nil {
			return resource.Resource{}, err
		}
		if hasPlaceholders {
			return resource.Resource{}, secrets.ErrSecretStoreNotConfigured
		}
		return data, nil
	}
	path := record.Path
	if strings.TrimSpace(path) == "" {
		path = "/"
	}
	return secrets.ResolveResourceSecrets(data, path, info.SecretInAttributes, r.SecretsManager)
}

func (r *DefaultReconciler) findRemotePathByAlias(record resource.ResourceRecord, desired resource.Resource, replacements map[string]string) (string, resource.Resource, bool, error) {
	if r.ResourceServerManager == nil {
		return "", resource.Resource{}, false, errors.New("resource server manager is not configured")
	}

	if record.Meta.ResourceInfo == nil {
		return "", resource.Resource{}, false, nil
	}

	aliasAttr := strings.TrimSpace(record.Meta.ResourceInfo.AliasFromAttribute)
	if aliasAttr == "" {
		return "", resource.Resource{}, false, nil
	}

	desiredAlias := resource.LastSegment(record.AliasPath(desired))
	if desiredAlias == "" {
		desiredAlias = resource.LastSegment(record.Path)
	}

	items, err := r.fetchCollection(record, replacePathSegments(record.CollectionPath(), replacements), false)
	if err != nil {
		return "", resource.Resource{}, false, err
	}

	for _, item := range items {
		alias := resource.LastSegment(record.AliasPath(item))
		if alias != desiredAlias {
			continue
		}
		return record.RemoteResourcePath(item), item, true, nil
	}

	return "", resource.Resource{}, false, nil
}

func (r *DefaultReconciler) findResourceInCollection(record resource.ResourceRecord, replacements map[string]string, target string) (resource.Resource, bool, error) {
	if r.ResourceServerManager == nil {
		return resource.Resource{}, false, errors.New("resource server manager is not configured")
	}
	target = strings.TrimSpace(target)
	if target == "" {
		return resource.Resource{}, false, nil
	}

	items, err := r.fetchCollection(record, replacePathSegments(record.CollectionPath(), replacements), false)
	if err != nil {
		return resource.Resource{}, false, err
	}

	listOp := adjustOperation(record.ReadOperation(true))

	idMatches := func(res resource.Resource) bool {
		if record.Meta.ResourceInfo == nil {
			return false
		}
		idAttr := strings.TrimSpace(record.Meta.ResourceInfo.IDFromAttribute)
		if idAttr == "" {
			return false
		}
		if value, ok := resource.LookupValueFromResource(res, idAttr); ok && strings.TrimSpace(value) != "" {
			return value == target
		}
		return false
	}
	aliasMatches := func(res resource.Resource) bool {
		if record.Meta.ResourceInfo == nil {
			return false
		}
		aliasAttr := strings.TrimSpace(record.Meta.ResourceInfo.AliasFromAttribute)
		if aliasAttr == "" {
			return false
		}
		if value, ok := resource.LookupValueFromResource(res, aliasAttr); ok && strings.TrimSpace(value) != "" {
			return value == target
		}
		return false
	}

	for _, item := range items {
		switch {
		case idMatches(item):
			return item, true, nil
		case aliasMatches(item):
			return item, true, nil
		}
	}

	if listOp != nil && strings.TrimSpace(listOp.JQFilter) != "" {
		opNoFilter := *record.ReadOperation(true)
		opNoFilter.JQFilter = ""
		opAdjusted := adjustOperation(&opNoFilter)
		specNoFilter, err := r.buildRequestSpecWithTarget(record, record.CollectionPath(), record.Path, opAdjusted, true)
		if err != nil {
			return resource.Resource{}, false, err
		}
		items, err := r.ResourceServerManager.GetResourceCollection(specNoFilter)
		if err != nil {
			return resource.Resource{}, false, err
		}
		for _, item := range items {
			switch {
			case idMatches(item):
				return item, true, nil
			case aliasMatches(item):
				return item, true, nil
			}
		}
	}

	return resource.Resource{}, false, nil
}

func (r *DefaultReconciler) fetchCollection(record resource.ResourceRecord, targetPath string, applyPayload bool) ([]resource.Resource, error) {
	op := adjustOperation(record.ReadOperation(true))
	spec, err := r.buildRequestSpecWithTarget(record, targetPath, record.Path, op, true)
	if err != nil {
		return nil, err
	}

	items, err := r.ResourceServerManager.GetResourceCollection(spec)
	if err != nil {
		return nil, err
	}

	items, err = applyCollectionFilter(op, items)
	if err != nil {
		return nil, err
	}

	if !applyPayload {
		return items, nil
	}

	payload := record.ListPayload()
	var processed []resource.Resource
	for _, item := range items {
		transformed, err := record.ApplyPayload(item, payload)
		if err != nil {
			return nil, err
		}
		processed = append(processed, transformed)
	}
	return processed, nil
}

func (r *DefaultReconciler) resolveRemoteResourcePath(path string, data resource.Resource) (string, resource.ResourceRecord, map[string]string, error) {
	record, err := r.recordFor(path)
	if err != nil {
		return path, resource.ResourceRecord{}, nil, err
	}

	replacements, err := r.buildAliasReplacements(path, data, true, true)
	if err != nil {
		return path, record, nil, err
	}

	resData, _, err := r.loadResourceForRemote(path, data, true)
	if err != nil {
		return path, record, nil, err
	}
	record.Data = resData

	collectionPath := replacePathSegments(record.CollectionPath(), replacements)
	if record.Meta.ResourceInfo != nil {
		record.Meta.ResourceInfo.CollectionPath = collectionPath
	}

	remotePath := replacePathSegments(record.RemoteResourcePath(resData), replacements)
	return remotePath, record, replacements, nil
}

func (r *DefaultReconciler) resolveRemoteCollectionPath(path string) (string, resource.ResourceRecord, map[string]string, error) {
	record, err := r.recordFor(path)
	if err != nil {
		return path, resource.ResourceRecord{}, nil, err
	}

	replacements, err := r.buildAliasReplacements(path, resource.Resource{}, false, true)
	if err != nil {
		return path, record, nil, err
	}

	collectionPath := replacePathSegments(record.CollectionPath(), replacements)
	if record.Meta.ResourceInfo != nil {
		record.Meta.ResourceInfo.CollectionPath = collectionPath
	}

	return collectionPath, record, replacements, nil
}

func (r *DefaultReconciler) buildAliasReplacements(path string, data resource.Resource, includeTarget bool, allowRemote bool) (map[string]string, error) {
	segments := resource.SplitPathSegments(path)
	replacements := make(map[string]string)
	limit := len(segments)
	if !includeTarget && limit > 0 {
		limit--
	}

	for i := 1; i <= limit; i++ {
		prefix := "/" + strings.Join(segments[:i], "/")
		isTarget := i == len(segments)

		resData, ok, err := r.loadResourceForRemote(prefix, data, isTarget && includeTarget)
		if err != nil {
			return nil, err
		}
		record, err := r.recordFor(prefix)
		if err != nil {
			return nil, err
		}

		if ok {
			if id := resource.LastSegment(record.RemoteResourcePath(resData)); id != "" {
				replacements[segments[i-1]] = id
			}
			continue
		}
		if !allowRemote || r.ResourceServerManager == nil || !shouldResolveRemoteAlias(record.Meta.ResourceInfo) {
			continue
		}
		resolved, matched, err := r.findResourceInCollection(record, replacements, segments[i-1])
		if err != nil {
			continue
		}
		if !matched {
			continue
		}
		if id := resource.LastSegment(record.RemoteResourcePath(resolved)); id != "" {
			replacements[segments[i-1]] = id
		}
	}

	return replacements, nil
}

func shouldResolveRemoteAlias(info *resource.ResourceInfoMetadata) bool {
	if info == nil {
		return false
	}
	alias := strings.TrimSpace(info.AliasFromAttribute)
	id := strings.TrimSpace(info.IDFromAttribute)
	if alias == "" || id == "" {
		return false
	}
	return alias != id
}

func (r *DefaultReconciler) loadResourceForRemote(path string, provided resource.Resource, isTarget bool) (resource.Resource, bool, error) {
	if isTarget && provided.V != nil {
		return provided, true, nil
	}

	if r != nil {
		if res, err := r.GetLocalResource(path); err == nil {
			return res, true, nil
		} else if !errors.Is(err, fs.ErrNotExist) {
			return resource.Resource{}, false, err
		}
	}

	if isTarget {
		return resource.Resource{}, true, nil
	}

	return resource.Resource{}, false, nil
}

func adjustOperation(op *resource.OperationMetadata) *resource.OperationMetadata {
	if op == nil {
		return nil
	}

	cloned := &resource.OperationMetadata{
		HTTPMethod:  op.HTTPMethod,
		HTTPHeaders: append(resource.HeaderList{}, op.HTTPHeaders...),
		JQFilter:    op.JQFilter,
	}

	if op.URL != nil {
		urlCopy := *op.URL
		if len(op.URL.QueryStrings) > 0 {
			urlCopy.QueryStrings = append([]string{}, op.URL.QueryStrings...)
		}
		cloned.URL = &urlCopy
	}

	if op.Payload != nil {
		cloned.Payload = clonePayloadConfig(op.Payload)
	}

	return cloned
}

func clonePayloadConfig(src *resource.OperationPayloadConfig) *resource.OperationPayloadConfig {
	if src == nil {
		return nil
	}
	return &resource.OperationPayloadConfig{
		SuppressAttributes: append([]string{}, src.SuppressAttributes...),
		FilterAttributes:   append([]string{}, src.FilterAttributes...),
		JQExpression:       src.JQExpression,
	}
}

func replacePathSegments(path string, replacements map[string]string) string {
	if len(replacements) == 0 {
		return path
	}

	raw := strings.TrimSpace(path)
	if raw == "" {
		return path
	}

	absolute := strings.HasPrefix(raw, "/")
	trimmed := strings.Trim(raw, "/")
	if trimmed == "" {
		if absolute {
			return "/"
		}
		return raw
	}

	parts := strings.Split(trimmed, "/")
	for idx, part := range parts {
		if replacement, ok := replacements[part]; ok && strings.TrimSpace(replacement) != "" {
			parts[idx] = replacement
		}
	}

	result := strings.Join(parts, "/")
	if absolute {
		return resource.NormalizePath("/" + result)
	}
	return result
}
