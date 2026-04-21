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

package controllers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	"github.com/crmarques/declarest/internal/bootstrap"
	"golang.org/x/sync/singleflight"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	k8sevents "k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	bundleReferencedByManagedService = "ReferencedByManagedService"
	bundleReferencedByCRDGenerator   = "ReferencedByCRDGenerator"
	bundleResolveFailed              = "ResolveFailed"

	metadataBundleIndexManagedService = "spec.metadata.bundleRef.name"
	metadataBundleIndexCRDGenerator   = "spec.versions.metadataBundleRef.name"
)

// MetadataBundleReconciler reconciles MetadataBundle resources by routing the
// configured source through the existing `bundlemetadata.ResolveBundle`
// pipeline and publishing the resolved manifest in status.
type MetadataBundleReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	Recorder                k8sevents.EventRecorder
	MaxConcurrentReconciles int

	// CacheRoot is the directory the resolver uses to cache extracted
	// bundles. When empty, the resolver falls back to
	// ~/.declarest/metadata-bundles. Operator deployments SHOULD set this to
	// a path on a writable volume (for example
	// "$DECLAREST_OPERATOR_CACHE_BASE_DIR/bundles").
	CacheRoot string

	// resolveGroup deduplicates concurrent resolutions for the same bundle
	// identity to avoid stampeding the origin (e.g. GitHub releases, OCI
	// registries) from parallel reconciles of the same CR.
	resolveGroup singleflight.Group

	// refCache maps MetadataBundle NamespacedName to the Secret and
	// ConfigMap names it currently references, so the Watch mappers can
	// enqueue the right bundles in O(1) when a credential or ConfigMap
	// rotates.
	refCache bundleRefCache
}

// bundleRefCache indexes per-bundle Secret and ConfigMap dependencies.
// Modelled after the syncpolicy secretRefCache pattern.
type bundleRefCache struct {
	mu         sync.RWMutex
	secrets    map[types.NamespacedName]sets.Set[string]
	configMaps map[types.NamespacedName]sets.Set[string]
}

func (c *bundleRefCache) update(key types.NamespacedName, secretNames []string, configMapNames []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.secrets == nil {
		c.secrets = make(map[types.NamespacedName]sets.Set[string])
	}
	if c.configMaps == nil {
		c.configMaps = make(map[types.NamespacedName]sets.Set[string])
	}
	if len(secretNames) == 0 {
		delete(c.secrets, key)
	} else {
		c.secrets[key] = sets.New(secretNames...)
	}
	if len(configMapNames) == 0 {
		delete(c.configMaps, key)
	} else {
		c.configMaps[key] = sets.New(configMapNames...)
	}
}

func (c *bundleRefCache) remove(key types.NamespacedName) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.secrets, key)
	delete(c.configMaps, key)
}

func (c *bundleRefCache) bundlesForSecret(namespace string, secretName string) []reconcile.Request {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var requests []reconcile.Request
	for key, names := range c.secrets {
		if key.Namespace == namespace && names.Has(secretName) {
			requests = append(requests, reconcile.Request{NamespacedName: key})
		}
	}
	return requests
}

func (c *bundleRefCache) bundlesForConfigMap(namespace string, configMapName string) []reconcile.Request {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var requests []reconcile.Request
	for key, names := range c.configMaps {
		if key.Namespace == namespace && names.Has(configMapName) {
			requests = append(requests, reconcile.Request{NamespacedName: key})
		}
	}
	return requests
}

type bundleResolutionResult struct {
	cachePath   string
	openAPIPath string
	manifest    declarestv1alpha1.MetadataBundleManifest
	warning     string
}

func (r *MetadataBundleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("metadataBundle", req.String(), "reconcile_id", uuidString())

	bundle := &declarestv1alpha1.MetadataBundle{}
	if err := r.Get(ctx, req.NamespacedName, bundle); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(bundle, finalizerName) {
		controllerutil.AddFinalizer(bundle, finalizerName)
		if err := r.Update(ctx, bundle); err != nil {
			return ctrl.Result{}, err
		}
	}

	pollInterval := metadataBundlePollInterval(bundle)

	if !bundle.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, bundle, pollInterval)
	}

	runtimeBundle := bundle.DeepCopy()
	runtimeBundle.Default()
	if err := runtimeBundle.ValidateSpec(); err != nil {
		emitEventf(r.Recorder, bundle, corev1.EventTypeWarning, "SpecInvalid", "validation failed: %v", err)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason, message string) error {
				return r.setNotReady(innerCtx, bundle, reason, message)
			},
			conditionReasonSpecInvalid,
			err.Error(),
			pollInterval,
		)
	}

	ref, err := metadataBundleSourceRef(runtimeBundle)
	if err != nil {
		emitEventf(r.Recorder, bundle, corev1.EventTypeWarning, bundleResolveFailed, "source invalid: %v", err)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason, message string) error {
				return r.setNotReady(innerCtx, bundle, reason, message)
			},
			conditionReasonSpecInvalid,
			err.Error(),
			pollInterval,
		)
	}

	sourceInputs, err := r.loadBundleSourceInputs(ctx, runtimeBundle)
	if err != nil {
		emitEventf(r.Recorder, bundle, corev1.EventTypeWarning, bundleResolveFailed, "dependency invalid: %v", err)
		r.refCache.update(req.NamespacedName, sourceInputs.SecretNames, sourceInputs.ConfigMapNames)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason, message string) error {
				return r.setNotReady(innerCtx, bundle, reason, message)
			},
			conditionReasonDependencyInvalid,
			err.Error(),
			pollInterval,
		)
	}
	r.refCache.update(req.NamespacedName, sourceInputs.SecretNames, sourceInputs.ConfigMapNames)

	resolution, err := r.resolveBundle(ctx, bundle, ref, sourceInputs)
	if err != nil {
		emitEventf(r.Recorder, bundle, corev1.EventTypeWarning, bundleResolveFailed, "resolve failed: %v", err)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason, message string) error {
				return r.setNotReady(innerCtx, bundle, reason, message)
			},
			bundleResolveFailed,
			err.Error(),
			pollInterval,
		)
	}

	bundle.Status.ObservedGeneration = bundle.Generation
	manifest := resolution.manifest
	bundle.Status.Manifest = &manifest
	bundle.Status.CachePath = resolution.cachePath
	bundle.Status.OpenAPIPath = resolution.openAPIPath
	resolvedAt := metav1.NewTime(time.Now().UTC())
	bundle.Status.LastResolvedTime = &resolvedAt
	bundle.Status.Conditions = setStatusCondition(
		bundle.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionTrue,
		conditionReasonReady,
		"bundle resolved",
	)
	bundle.Status.Conditions = setStatusCondition(
		bundle.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionFalse,
		conditionReasonReady,
		"",
	)
	if err := r.Status().Update(ctx, bundle); err != nil {
		return ctrl.Result{}, err
	}

	if resolution.warning != "" {
		emitEventf(r.Recorder, bundle, corev1.EventTypeWarning, "BundleDeprecated", "%s", resolution.warning)
	}

	logger.Info(
		"metadata bundle reconciled",
		"name", manifest.Name,
		"version", manifest.Version,
		"cache_path", shortenPath(resolution.cachePath),
		"poll_interval", pollInterval.String(),
	)
	return ctrl.Result{RequeueAfter: pollInterval}, nil
}

func (r *MetadataBundleReconciler) handleDeletion(
	ctx context.Context,
	bundle *declarestv1alpha1.MetadataBundle,
	pollInterval time.Duration,
) (ctrl.Result, error) {
	dependents, err := r.listDependents(ctx, bundle)
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(dependents) > 0 {
		reason, message := dependentsReason(dependents)
		emitEventf(r.Recorder, bundle, corev1.EventTypeWarning, reason, "%s", message)
		if updateErr := r.setNotReady(ctx, bundle, reason, message); updateErr != nil {
			return ctrl.Result{}, updateErr
		}
		if pollInterval <= 0 {
			pollInterval = defaultTransientRequeueInterval
		}
		return ctrl.Result{RequeueAfter: pollInterval}, nil
	}

	controllerutil.RemoveFinalizer(bundle, finalizerName)
	if err := r.Update(ctx, bundle); err != nil {
		return ctrl.Result{}, err
	}
	r.refCache.remove(types.NamespacedName{Namespace: bundle.Namespace, Name: bundle.Name})
	return ctrl.Result{}, nil
}

type metadataBundleDependent struct {
	Kind string
	Name string
}

func (r *MetadataBundleReconciler) listDependents(
	ctx context.Context,
	bundle *declarestv1alpha1.MetadataBundle,
) ([]metadataBundleDependent, error) {
	var dependents []metadataBundleDependent

	services := &declarestv1alpha1.ManagedServiceList{}
	if err := r.List(ctx, services, client.InNamespace(bundle.Namespace)); err != nil {
		return nil, fmt.Errorf("list managed services: %w", err)
	}
	for idx := range services.Items {
		svc := &services.Items[idx]
		if svc.DeletionTimestamp != nil {
			continue
		}
		if ref := svc.Spec.Metadata.BundleRef; ref != nil && strings.TrimSpace(ref.Name) == bundle.Name {
			dependents = append(dependents, metadataBundleDependent{Kind: "ManagedService", Name: svc.Name})
		}
	}

	generators := &declarestv1alpha1.CRDGeneratorList{}
	if err := r.List(ctx, generators, client.InNamespace(bundle.Namespace)); err != nil {
		return nil, fmt.Errorf("list crd generators: %w", err)
	}
	for idx := range generators.Items {
		gen := &generators.Items[idx]
		if gen.DeletionTimestamp != nil {
			continue
		}
		for _, version := range gen.Spec.Versions {
			if strings.TrimSpace(version.MetadataBundleRef.Name) == bundle.Name {
				dependents = append(dependents, metadataBundleDependent{Kind: "CRDGenerator", Name: gen.Name + ":" + version.Name})
				break
			}
		}
	}
	return dependents, nil
}

func dependentsReason(dependents []metadataBundleDependent) (string, string) {
	kinds := map[string]bool{}
	names := make([]string, 0, len(dependents))
	for _, dep := range dependents {
		kinds[dep.Kind] = true
		names = append(names, dep.Kind+"/"+dep.Name)
	}
	reason := bundleReferencedByManagedService
	if kinds["CRDGenerator"] && !kinds["ManagedService"] {
		reason = bundleReferencedByCRDGenerator
	}
	return reason, fmt.Sprintf("metadata bundle is still referenced by %s", strings.Join(names, ", "))
}

func metadataBundleSourceRef(bundle *declarestv1alpha1.MetadataBundle) (string, error) {
	source := bundle.Spec.Source
	if value := strings.TrimSpace(source.URL); value != "" {
		return value, nil
	}
	if source.PVC != nil {
		return filepath.Clean(filepath.Join(resolveBundlePVCRootPath(bundle), source.PVC.Path)), nil
	}
	if source.ConfigMap != nil {
		name := strings.TrimSpace(source.ConfigMap.Name)
		key := strings.TrimSpace(source.ConfigMap.Key)
		if name == "" || key == "" {
			return "", fmt.Errorf("bundle configMap source is missing name or key")
		}
		return fmt.Sprintf("configmap://%s/%s/%s", bundle.Namespace, name, key), nil
	}
	return "", fmt.Errorf("bundle source is empty")
}

// resolveBundlePVCRootPath maps the configured PVC to a canonical filesystem
// root. When the operator runs in-cluster the PVC is mounted at
// /var/lib/declarest/bundles/<namespace>/<pvc>; otherwise the root falls back
// to the cache base directory so developer runs outside the cluster still
// work.
func resolveBundlePVCRootPath(bundle *declarestv1alpha1.MetadataBundle) string {
	if bundle == nil || bundle.Spec.Source.PVC == nil {
		return ""
	}
	pvcName := ""
	if bundle.Spec.Source.PVC.Storage.ExistingPVC != nil {
		pvcName = bundle.Spec.Source.PVC.Storage.ExistingPVC.Name
	}
	if pvcName == "" {
		pvcName = bundle.Name
	}
	return filepath.Join("/var/lib/declarest/bundles", bundle.Namespace, pvcName)
}

// bundleSourceInputs carries the runtime-resolved dependencies (registry
// credentials, in-memory tarball bytes) and the resourceVersion fingerprints
// used to invalidate singleflight dedup when the underlying Secret or
// ConfigMap rotates.
type bundleSourceInputs struct {
	RegistryAuths    []bootstrap.RegistryAuth
	InMemoryBundles  map[string][]byte
	SecretNames      []string
	ConfigMapNames   []string
	DependencyDigest string
}

func (r *MetadataBundleReconciler) loadBundleSourceInputs(
	ctx context.Context,
	bundle *declarestv1alpha1.MetadataBundle,
) (bundleSourceInputs, error) {
	inputs := bundleSourceInputs{}
	source := bundle.Spec.Source

	if source.PullSecretRef != nil && strings.TrimSpace(source.PullSecretRef.Name) != "" &&
		strings.HasPrefix(strings.ToLower(strings.TrimSpace(source.URL)), "oci://") {
		secretName := strings.TrimSpace(source.PullSecretRef.Name)
		secret := &corev1.Secret{}
		key := types.NamespacedName{Namespace: bundle.Namespace, Name: secretName}
		if err := r.Get(ctx, key, secret); err != nil {
			return inputs, fmt.Errorf("load oci pull secret %s/%s: %w", bundle.Namespace, secretName, err)
		}
		if secret.Type != corev1.SecretTypeDockerConfigJson {
			return inputs, fmt.Errorf(
				"oci pull secret %s/%s must be of type %s, got %s",
				bundle.Namespace, secretName, corev1.SecretTypeDockerConfigJson, secret.Type,
			)
		}
		payload, ok := secret.Data[corev1.DockerConfigJsonKey]
		if !ok || len(payload) == 0 {
			return inputs, fmt.Errorf(
				"oci pull secret %s/%s is missing %q",
				bundle.Namespace, secretName, corev1.DockerConfigJsonKey,
			)
		}
		auths, err := parseDockerConfigAuths(payload)
		if err != nil {
			return inputs, fmt.Errorf("parse oci pull secret %s/%s: %w", bundle.Namespace, secretName, err)
		}
		if len(auths) == 0 {
			return inputs, fmt.Errorf("oci pull secret %s/%s has no usable auths entry", bundle.Namespace, secretName)
		}
		inputs.RegistryAuths = auths
		inputs.SecretNames = []string{secretName}
		inputs.DependencyDigest = "secret:" + secretName + ":" + secret.ResourceVersion
	}

	if source.ConfigMap != nil {
		cmName := strings.TrimSpace(source.ConfigMap.Name)
		cmKey := strings.TrimSpace(source.ConfigMap.Key)
		if cmName == "" || cmKey == "" {
			return inputs, fmt.Errorf("configMap source name and key are required")
		}
		cm := &corev1.ConfigMap{}
		key := types.NamespacedName{Namespace: bundle.Namespace, Name: cmName}
		if err := r.Get(ctx, key, cm); err != nil {
			return inputs, fmt.Errorf("load configMap %s/%s: %w", bundle.Namespace, cmName, err)
		}
		data, err := extractConfigMapBundleBytes(cm, cmKey)
		if err != nil {
			return inputs, err
		}
		inMemoryKey := fmt.Sprintf("configmap://%s/%s/%s", bundle.Namespace, cmName, cmKey)
		inputs.InMemoryBundles = map[string][]byte{inMemoryKey: data}
		inputs.ConfigMapNames = []string{cmName}
		if existing := inputs.DependencyDigest; existing != "" {
			inputs.DependencyDigest = existing + "|"
		}
		inputs.DependencyDigest += "configmap:" + cmName + ":" + cm.ResourceVersion
	}

	return inputs, nil
}

// parseDockerConfigAuths decodes a kubernetes.io/dockerconfigjson payload
// into bootstrap.RegistryAuth entries. Hosts with both a separate
// username/password and an `auth` (base64 user:pass) field prefer the former
// when present, falling back to decoding `auth` otherwise.
func parseDockerConfigAuths(payload []byte) ([]bootstrap.RegistryAuth, error) {
	var config struct {
		Auths map[string]struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Auth     string `json:"auth"`
		} `json:"auths"`
	}
	if err := json.Unmarshal(payload, &config); err != nil {
		return nil, fmt.Errorf("decode dockerconfigjson: %w", err)
	}
	out := make([]bootstrap.RegistryAuth, 0, len(config.Auths))
	for registry, entry := range config.Auths {
		registry = strings.TrimSpace(registry)
		if registry == "" {
			continue
		}
		username := entry.Username
		password := entry.Password
		if username == "" && password == "" && strings.TrimSpace(entry.Auth) != "" {
			decoded, err := base64.StdEncoding.DecodeString(strings.TrimSpace(entry.Auth))
			if err != nil {
				return nil, fmt.Errorf("decode auth for registry %q: %w", registry, err)
			}
			parts := strings.SplitN(string(decoded), ":", 2)
			if len(parts) != 2 {
				return nil, fmt.Errorf("auth for registry %q is not in user:password form", registry)
			}
			username, password = parts[0], parts[1]
		}
		if username == "" && password == "" {
			continue
		}
		out = append(out, bootstrap.RegistryAuth{
			Registry: normalizeRegistryHost(registry),
			Username: username,
			Password: password,
		})
	}
	return out, nil
}

// normalizeRegistryHost strips URL scheme and path components commonly found
// in dockerconfigjson keys (for example "https://index.docker.io/v1/").
func normalizeRegistryHost(raw string) string {
	value := strings.TrimSpace(raw)
	value = strings.TrimPrefix(value, "https://")
	value = strings.TrimPrefix(value, "http://")
	if idx := strings.Index(value, "/"); idx >= 0 {
		value = value[:idx]
	}
	return strings.ToLower(value)
}

// extractConfigMapBundleBytes returns the gzipped tarball bytes stored in the
// referenced ConfigMap key. binaryData takes precedence; data is base64-
// decoded when binaryData is unset so `kubectl create configmap
// --from-file=<key>=<tarball>` just works for small bundles.
func extractConfigMapBundleBytes(cm *corev1.ConfigMap, key string) ([]byte, error) {
	if cm == nil {
		return nil, fmt.Errorf("configMap is nil")
	}
	if bytes, ok := cm.BinaryData[key]; ok {
		if len(bytes) == 0 {
			return nil, fmt.Errorf("configMap %s/%s key %q is empty", cm.Namespace, cm.Name, key)
		}
		return bytes, nil
	}
	if encoded, ok := cm.Data[key]; ok {
		trimmed := strings.TrimSpace(encoded)
		if trimmed == "" {
			return nil, fmt.Errorf("configMap %s/%s key %q is empty", cm.Namespace, cm.Name, key)
		}
		decoded, err := base64.StdEncoding.DecodeString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("decode configMap %s/%s key %q: %w", cm.Namespace, cm.Name, key, err)
		}
		return decoded, nil
	}
	return nil, fmt.Errorf("configMap %s/%s has no key %q", cm.Namespace, cm.Name, key)
}

func (r *MetadataBundleReconciler) resolveBundle(
	ctx context.Context,
	bundle *declarestv1alpha1.MetadataBundle,
	ref string,
	inputs bundleSourceInputs,
) (*bundleResolutionResult, error) {
	key := fmt.Sprintf("%s|%s|%s", bundle.Namespace, ref, inputs.DependencyDigest)
	value, err, _ := r.resolveGroup.Do(key, func() (any, error) {
		resolveOpts := bootstrap.MetadataBundleResolveOptions{
			CacheRoot:       strings.TrimSpace(r.CacheRoot),
			RegistryAuths:   inputs.RegistryAuths,
			InMemoryBundles: inputs.InMemoryBundles,
		}
		resolution, resolveErr := bootstrap.ResolveMetadataBundle(ctx, ref, resolveOpts)
		if resolveErr != nil {
			return nil, resolveErr
		}
		return &resolution, nil
	})
	if err != nil {
		return nil, err
	}
	resolution, ok := value.(*bootstrap.ResolvedMetadataBundle)
	if !ok || resolution == nil {
		return nil, fmt.Errorf("bundle resolution returned unexpected value type")
	}
	result := &bundleResolutionResult{
		cachePath:   resolution.MetadataDir,
		openAPIPath: strings.TrimSpace(resolution.OpenAPI),
		manifest:    convertResolvedBundle(*resolution),
		warning:     resolution.DeprecatedWarning,
	}
	if result.manifest.OpenAPI == "" && strings.TrimSpace(resolution.OpenAPI) != "" {
		result.manifest.OpenAPI = resolution.OpenAPI
	}
	return result, nil
}

func convertResolvedBundle(resolved bootstrap.ResolvedMetadataBundle) declarestv1alpha1.MetadataBundleManifest {
	result := declarestv1alpha1.MetadataBundleManifest{
		Name:                resolved.Name,
		Version:             resolved.Version,
		Description:         resolved.Description,
		MetadataRoot:        resolved.MetadataRoot,
		OpenAPI:             resolved.DeclarestOpenAPI,
		CompatibleDeclarest: resolved.CompatibleDeclarest,
		Deprecated:          resolved.Deprecated,
	}
	if strings.TrimSpace(resolved.CompatibleProduct) != "" ||
		strings.TrimSpace(resolved.CompatibleVersions) != "" {
		result.CompatibleManagedService = &declarestv1alpha1.MetadataBundleCompatibility{
			Product:  resolved.CompatibleProduct,
			Versions: resolved.CompatibleVersions,
		}
	}
	return result
}

func metadataBundlePollInterval(bundle *declarestv1alpha1.MetadataBundle) time.Duration {
	if bundle == nil || bundle.Spec.PollInterval == nil || bundle.Spec.PollInterval.Duration <= 0 {
		return time.Hour
	}
	return bundle.Spec.PollInterval.Duration
}

func (r *MetadataBundleReconciler) setNotReady(
	ctx context.Context,
	bundle *declarestv1alpha1.MetadataBundle,
	reason string,
	message string,
) error {
	bundle.Status.ObservedGeneration = bundle.Generation
	bundle.Status.Conditions = setStatusCondition(
		bundle.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionFalse,
		reason,
		message,
	)
	bundle.Status.Conditions = setStatusCondition(
		bundle.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionTrue,
		reason,
		message,
	)
	return r.Status().Update(ctx, bundle)
}

func (r *MetadataBundleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&declarestv1alpha1.ManagedService{},
		metadataBundleIndexManagedService,
		func(obj client.Object) []string {
			svc, ok := obj.(*declarestv1alpha1.ManagedService)
			if !ok || svc.Spec.Metadata.BundleRef == nil {
				return nil
			}
			name := strings.TrimSpace(svc.Spec.Metadata.BundleRef.Name)
			if name == "" {
				return nil
			}
			return []string{name}
		},
	); err != nil {
		return fmt.Errorf("index ManagedService bundleRef: %w", err)
	}
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&declarestv1alpha1.CRDGenerator{},
		metadataBundleIndexCRDGenerator,
		func(obj client.Object) []string {
			gen, ok := obj.(*declarestv1alpha1.CRDGenerator)
			if !ok {
				return nil
			}
			var names []string
			for _, version := range gen.Spec.Versions {
				if name := strings.TrimSpace(version.MetadataBundleRef.Name); name != "" {
					names = append(names, name)
				}
			}
			return names
		},
	); err != nil {
		return fmt.Errorf("index CRDGenerator bundleRef: %w", err)
	}

	bld := ctrl.NewControllerManagedBy(mgr).
		For(&declarestv1alpha1.MetadataBundle{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(r.metadataBundlesForSecret)).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(r.metadataBundlesForConfigMap))
	if r.MaxConcurrentReconciles > 0 {
		bld = bld.WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles})
	}
	return bld.Complete(r)
}

func (r *MetadataBundleReconciler) metadataBundlesForSecret(_ context.Context, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok || secret == nil {
		return nil
	}
	return r.refCache.bundlesForSecret(secret.Namespace, secret.Name)
}

func (r *MetadataBundleReconciler) metadataBundlesForConfigMap(_ context.Context, obj client.Object) []reconcile.Request {
	configMap, ok := obj.(*corev1.ConfigMap)
	if !ok || configMap == nil {
		return nil
	}
	return r.refCache.bundlesForConfigMap(configMap.Namespace, configMap.Name)
}
