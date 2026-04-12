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
	"os"
	"time"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
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

// ManagedServiceReconciler reconciles ManagedService resources.
type ManagedServiceReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	Recorder                k8sevents.EventRecorder
	MaxConcurrentReconciles int
}

func (r *ManagedServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("managedService", req.String(), "reconcile_id", uuidString())
	managedService := &declarestv1alpha1.ManagedService{}
	if err := r.Get(ctx, req.NamespacedName, managedService); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(managedService, finalizerName) {
		controllerutil.AddFinalizer(managedService, finalizerName)
		if err := r.Update(ctx, managedService); err != nil {
			return ctrl.Result{}, err
		}
	}
	if !managedService.DeletionTimestamp.IsZero() {
		cacheDir := resolveCacheRootPath(managedService.Namespace, managedService.Name)
		if err := os.RemoveAll(cacheDir); err != nil {
			logger.Error(err, "failed to clean up cache directory", "path", cacheDir)
		}
		controllerutil.RemoveFinalizer(managedService, finalizerName)
		return ctrl.Result{}, r.Update(ctx, managedService)
	}

	runtimeManagedService := expandRuntimeManagedService(managedService)
	runtimeManagedService.Default()
	pollInterval := managedServicePollInterval(runtimeManagedService)
	if validationErr := runtimeManagedService.ValidateSpec(); validationErr != nil {
		logger.Error(validationErr, "managed service spec validation failed")
		emitEventf(r.Recorder, managedService, corev1.EventTypeWarning, "SpecInvalid", "validation failed: %v", validationErr)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason string, message string) error {
				return r.setNotReady(innerCtx, managedService, reason, message)
			},
			conditionReasonSpecInvalid,
			validationErr.Error(),
			pollInterval,
		)
	}

	cacheDir := resolveCacheRootPath(managedService.Namespace, managedService.Name)
	proxyConfig, proxyErr := resolveManagedServiceProxyConfig(ctx, r.Client, managedService.Namespace, runtimeManagedService.Spec.HTTP.Proxy)
	if proxyErr != nil {
		emitEventf(r.Recorder, managedService, corev1.EventTypeWarning, "ProxyConfigFailed", "proxy configuration failed: %v", proxyErr)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason string, message string) error {
				return r.setNotReady(innerCtx, managedService, reason, message)
			},
			conditionReasonDependencyInvalid,
			proxyErr.Error(),
			pollInterval,
		)
	}

	openAPIPath, openAPIErr := downloadArtifact(ctx, runtimeManagedService.Spec.OpenAPI.URL, cacheDir, proxyConfig)
	if openAPIErr != nil {
		emitEventf(r.Recorder, managedService, corev1.EventTypeWarning, "DownloadFailed", "openapi artifact download failed: %v", openAPIErr)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason string, message string) error {
				return r.setNotReady(innerCtx, managedService, reason, message)
			},
			conditionReasonDependencyInvalid,
			openAPIErr.Error(),
			pollInterval,
		)
	}
	metadataPath, metadataErr := downloadArtifact(ctx, runtimeManagedService.Spec.Metadata.URL, cacheDir, proxyConfig)
	if metadataErr != nil {
		emitEventf(r.Recorder, managedService, corev1.EventTypeWarning, "DownloadFailed", "metadata artifact download failed: %v", metadataErr)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason string, message string) error {
				return r.setNotReady(innerCtx, managedService, reason, message)
			},
			conditionReasonDependencyInvalid,
			metadataErr.Error(),
			pollInterval,
		)
	}

	previousOpenAPIPath := managedService.Status.OpenAPICachePath
	previousMetadataPath := managedService.Status.MetadataCachePath
	managedService.Status.ObservedGeneration = managedService.Generation
	managedService.Status.OpenAPICachePath = openAPIPath
	managedService.Status.MetadataCachePath = metadataPath
	managedService.Status.Conditions = setStatusCondition(
		managedService.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionTrue,
		conditionReasonReady,
		"managed service configuration is valid",
	)
	managedService.Status.Conditions = setStatusCondition(
		managedService.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionFalse,
		conditionReasonReady,
		"",
	)
	if err := r.Status().Update(ctx, managedService); err != nil {
		return ctrl.Result{}, err
	}
	if previousOpenAPIPath != openAPIPath || previousMetadataPath != metadataPath {
		emitEventf(
			r.Recorder,
			managedService,
			corev1.EventTypeNormal,
			"ArtifactsCached",
			"updated cached artifacts (openapi=%s metadata=%s)",
			shortenPath(openAPIPath),
			shortenPath(metadataPath),
		)
	}

	logger.Info(
		"managed service reconciled",
		"openapi", shortenPath(openAPIPath),
		"metadata", shortenPath(metadataPath),
		"poll_interval", pollInterval.String(),
	)
	return ctrl.Result{RequeueAfter: pollInterval}, nil
}

func managedServicePollInterval(managedService *declarestv1alpha1.ManagedService) time.Duration {
	if managedService == nil || managedService.Spec.PollInterval == nil || managedService.Spec.PollInterval.Duration <= 0 {
		return 10 * time.Minute
	}
	return managedService.Spec.PollInterval.Duration
}

func (r *ManagedServiceReconciler) setNotReady(
	ctx context.Context,
	managedService *declarestv1alpha1.ManagedService,
	reason string,
	message string,
) error {
	managedService.Status.ObservedGeneration = managedService.Generation
	managedService.Status.Conditions = setStatusCondition(
		managedService.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionFalse,
		reason,
		message,
	)
	managedService.Status.Conditions = setStatusCondition(
		managedService.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionTrue,
		reason,
		message,
	)
	return r.Status().Update(ctx, managedService)
}

func (r *ManagedServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	bld := ctrl.NewControllerManagedBy(mgr).
		For(&declarestv1alpha1.ManagedService{}, builder.WithPredicates(predicate.GenerationChangedPredicate{}))
	if r.MaxConcurrentReconciles > 0 {
		bld = bld.WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles})
	}
	return bld.Complete(r)
}

func shortenPath(value string) string {
	if value == "" {
		return ""
	}
	if len(value) <= 32 {
		return value
	}
	return fmt.Sprintf("...%s", value[len(value)-32:])
}
