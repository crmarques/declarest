package controllers

import (
	"context"
	"fmt"
	"os"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ManagedServerReconciler reconciles ManagedServer resources.
type ManagedServerReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *ManagedServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("managedServer", req.NamespacedName.String(), "reconcile_id", uuidString())
	managedServer := &declarestv1alpha1.ManagedServer{}
	if err := r.Get(ctx, req.NamespacedName, managedServer); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(managedServer, finalizerName) {
		controllerutil.AddFinalizer(managedServer, finalizerName)
		if err := r.Update(ctx, managedServer); err != nil {
			return ctrl.Result{}, err
		}
	}
	if !managedServer.DeletionTimestamp.IsZero() {
		cacheDir := resolveCacheRootPath(managedServer.Namespace, managedServer.Name)
		_ = os.RemoveAll(cacheDir)
		controllerutil.RemoveFinalizer(managedServer, finalizerName)
		return ctrl.Result{}, r.Update(ctx, managedServer)
	}

	managedServer.Default()
	if validationErr := managedServer.ValidateSpec(); validationErr != nil {
		logger.Error(validationErr, "managed server spec validation failed")
		emitEventf(r.Recorder, managedServer, corev1.EventTypeWarning, "SpecInvalid", "validation failed: %v", validationErr)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason string, message string) error {
				return r.setNotReady(innerCtx, managedServer, reason, message)
			},
			conditionReasonSpecInvalid,
			validationErr.Error(),
			managedServer.Spec.PollInterval.Duration,
		)
	}

	cacheDir := resolveCacheRootPath(managedServer.Namespace, managedServer.Name)
	openAPIPath, openAPIErr := downloadArtifact(ctx, managedServer.Spec.OpenAPI.URL, cacheDir)
	if openAPIErr != nil {
		emitEventf(r.Recorder, managedServer, corev1.EventTypeWarning, "DownloadFailed", "openapi artifact download failed: %v", openAPIErr)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason string, message string) error {
				return r.setNotReady(innerCtx, managedServer, reason, message)
			},
			conditionReasonDependencyInvalid,
			openAPIErr.Error(),
			managedServer.Spec.PollInterval.Duration,
		)
	}
	metadataPath, metadataErr := downloadArtifact(ctx, managedServer.Spec.Metadata.URL, cacheDir)
	if metadataErr != nil {
		emitEventf(r.Recorder, managedServer, corev1.EventTypeWarning, "DownloadFailed", "metadata artifact download failed: %v", metadataErr)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason string, message string) error {
				return r.setNotReady(innerCtx, managedServer, reason, message)
			},
			conditionReasonDependencyInvalid,
			metadataErr.Error(),
			managedServer.Spec.PollInterval.Duration,
		)
	}

	previousOpenAPIPath := managedServer.Status.OpenAPICachePath
	previousMetadataPath := managedServer.Status.MetadataCachePath
	managedServer.Status.ObservedGeneration = managedServer.Generation
	managedServer.Status.OpenAPICachePath = openAPIPath
	managedServer.Status.MetadataCachePath = metadataPath
	managedServer.Status.Conditions = setStatusCondition(
		managedServer.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionTrue,
		conditionReasonReady,
		"managed server configuration is valid",
	)
	managedServer.Status.Conditions = setStatusCondition(
		managedServer.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionFalse,
		conditionReasonReady,
		"",
	)
	if err := r.Status().Update(ctx, managedServer); err != nil {
		return ctrl.Result{}, err
	}
	if previousOpenAPIPath != openAPIPath || previousMetadataPath != metadataPath {
		emitEventf(
			r.Recorder,
			managedServer,
			corev1.EventTypeNormal,
			"ArtifactsCached",
			"updated cached artifacts (openapi=%s metadata=%s)",
			shortenPath(openAPIPath),
			shortenPath(metadataPath),
		)
	}

	logger.Info(
		"managed server reconciled",
		"openapi", shortenPath(openAPIPath),
		"metadata", shortenPath(metadataPath),
		"poll_interval", managedServer.Spec.PollInterval.Duration.String(),
	)
	return ctrl.Result{RequeueAfter: managedServer.Spec.PollInterval.Duration}, nil
}

func (r *ManagedServerReconciler) setNotReady(
	ctx context.Context,
	managedServer *declarestv1alpha1.ManagedServer,
	reason string,
	message string,
) error {
	managedServer.Status.ObservedGeneration = managedServer.Generation
	managedServer.Status.Conditions = setStatusCondition(
		managedServer.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionFalse,
		reason,
		message,
	)
	managedServer.Status.Conditions = setStatusCondition(
		managedServer.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionTrue,
		reason,
		message,
	)
	return r.Status().Update(ctx, managedServer)
}

func (r *ManagedServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&declarestv1alpha1.ManagedServer{}).
		Complete(r)
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
