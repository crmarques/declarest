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
	"fmt"
	"path/filepath"
	"strings"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	"github.com/crmarques/declarest/internal/bootstrap"
	"golang.org/x/sync/singleflight"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sevents "k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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

	// resolveGroup deduplicates concurrent resolutions for the same bundle
	// identity to avoid stampeding the origin (e.g. GitHub releases).
	resolveGroup singleflight.Group
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

	resolution, err := r.resolveBundle(ctx, bundle, ref)
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
	if value := strings.TrimSpace(source.Shorthand); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(source.URL); value != "" {
		return value, nil
	}
	if source.File != nil {
		return filepath.Clean(filepath.Join(resolveBundleFileRootPath(bundle), source.File.Path)), nil
	}
	return "", fmt.Errorf("bundle source is empty")
}

// resolveBundleFileRootPath maps the configured PVC to a canonical filesystem
// root. When the operator runs in-cluster the PVC is mounted at
// /var/lib/declarest/bundles/<namespace>/<pvc>; otherwise the root falls back
// to the cache base directory so developer runs outside the cluster still
// work.
func resolveBundleFileRootPath(bundle *declarestv1alpha1.MetadataBundle) string {
	if bundle == nil || bundle.Spec.Source.File == nil {
		return ""
	}
	pvcName := ""
	if bundle.Spec.Source.File.Storage.ExistingPVC != nil {
		pvcName = bundle.Spec.Source.File.Storage.ExistingPVC.Name
	}
	if pvcName == "" {
		pvcName = bundle.Name
	}
	return filepath.Join("/var/lib/declarest/bundles", bundle.Namespace, pvcName)
}

func (r *MetadataBundleReconciler) resolveBundle(
	ctx context.Context,
	bundle *declarestv1alpha1.MetadataBundle,
	ref string,
) (*bundleResolutionResult, error) {
	key := fmt.Sprintf("%s|%s", bundle.Namespace, ref)
	value, err, _ := r.resolveGroup.Do(key, func() (any, error) {
		resolution, resolveErr := bootstrap.ResolveMetadataBundle(ctx, ref)
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
		For(&declarestv1alpha1.MetadataBundle{}, builder.WithPredicates(predicate.GenerationChangedPredicate{}))
	if r.MaxConcurrentReconciles > 0 {
		bld = bld.WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles})
	}
	return bld.Complete(r)
}
