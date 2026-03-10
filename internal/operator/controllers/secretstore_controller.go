package controllers

import (
	"context"
	"fmt"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// SecretStoreReconciler reconciles SecretStore resources.
type SecretStoreReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
}

func (r *SecretStoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("secretStore", req.NamespacedName.String(), "reconcile_id", uuidString())
	secretStore := &declarestv1alpha1.SecretStore{}
	if err := r.Get(ctx, req.NamespacedName, secretStore); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !controllerutil.ContainsFinalizer(secretStore, finalizerName) {
		controllerutil.AddFinalizer(secretStore, finalizerName)
		if err := r.Update(ctx, secretStore); err != nil {
			return ctrl.Result{}, err
		}
	}
	if !secretStore.DeletionTimestamp.IsZero() {
		controllerutil.RemoveFinalizer(secretStore, finalizerName)
		return ctrl.Result{}, r.Update(ctx, secretStore)
	}

	runtimeSecretStore := expandRuntimeSecretStore(secretStore)
	if validationErr := runtimeSecretStore.ValidateSpec(); validationErr != nil {
		logger.Error(validationErr, "secret store spec validation failed")
		emitEventf(r.Recorder, secretStore, corev1.EventTypeWarning, "SpecInvalid", "validation failed: %v", validationErr)
		return returnAfterSetNotReady(
			ctx,
			func(innerCtx context.Context, reason string, message string) error {
				return r.setNotReady(innerCtx, secretStore, reason, message)
			},
			conditionReasonSpecInvalid,
			validationErr.Error(),
			0,
		)
	}

	resolvedPath := ""
	if runtimeSecretStore.Spec.File != nil {
		if err := r.ensureFilePVC(ctx, runtimeSecretStore); err != nil {
			emitEventf(r.Recorder, secretStore, corev1.EventTypeWarning, "DependencyInvalid", "dependency validation failed: %v", err)
			return returnAfterSetNotReady(
				ctx,
				func(innerCtx context.Context, reason string, message string) error {
					return r.setNotReady(innerCtx, secretStore, reason, message)
				},
				conditionReasonDependencyInvalid,
				err.Error(),
				0,
			)
		}
		resolvedPath = runtimeSecretStore.Spec.File.Path
	}

	secretStore.Status.ObservedGeneration = secretStore.Generation
	secretStore.Status.ResolvedPath = resolvedPath
	secretStore.Status.Conditions = setStatusCondition(
		secretStore.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionTrue,
		conditionReasonReady,
		"secret store configuration is valid",
	)
	secretStore.Status.Conditions = setStatusCondition(
		secretStore.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionFalse,
		conditionReasonReady,
		"",
	)
	if err := r.Status().Update(ctx, secretStore); err != nil {
		return ctrl.Result{}, err
	}
	emitEventf(r.Recorder, secretStore, corev1.EventTypeNormal, "Ready", "secret store configuration is valid")

	logger.Info("secret store reconciled")
	return ctrl.Result{}, nil
}

func (r *SecretStoreReconciler) ensureFilePVC(ctx context.Context, secretStore *declarestv1alpha1.SecretStore) error {
	if secretStore.Spec.File == nil {
		return nil
	}
	if secretStore.Spec.File.Storage.ExistingPVC != nil {
		name := secretStore.Spec.File.Storage.ExistingPVC.Name
		existing := &corev1.PersistentVolumeClaim{}
		if err := r.Get(ctx, types.NamespacedName{Namespace: secretStore.Namespace, Name: name}, existing); err != nil {
			return fmt.Errorf("resolve existing file PVC %q: %w", name, err)
		}
		return nil
	}
	if secretStore.Spec.File.Storage.PVC == nil {
		return fmt.Errorf("file storage pvc template is required")
	}
	pvcName := fmt.Sprintf("%s-secrets", secretStore.Name)
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Namespace: secretStore.Namespace, Name: pvcName}, pvc)
	if apierrors.IsNotFound(err) {
		pvc.Namespace = secretStore.Namespace
		pvc.Name = pvcName
		pvc.Labels = mergeStringMap(nil, map[string]string{
			"app.kubernetes.io/name":    "declarest-operator",
			"declarest.io/secret-store": secretStore.Name,
		})
		if refErr := controllerutil.SetControllerReference(secretStore, pvc, r.Scheme); refErr != nil {
			return refErr
		}
		pvc.Spec.AccessModes = append([]corev1.PersistentVolumeAccessMode(nil), secretStore.Spec.File.Storage.PVC.AccessModes...)
		pvc.Spec.Resources.Requests = secretStore.Spec.File.Storage.PVC.Requests.DeepCopy()
		pvc.Spec.StorageClassName = secretStore.Spec.File.Storage.PVC.StorageClassName
		if createErr := r.Create(ctx, pvc); createErr != nil {
			return fmt.Errorf("create secret store PVC %q: %w", pvcName, createErr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get secret store PVC %q: %w", pvcName, err)
	}
	return nil
}

func (r *SecretStoreReconciler) setNotReady(
	ctx context.Context,
	secretStore *declarestv1alpha1.SecretStore,
	reason string,
	message string,
) error {
	secretStore.Status.ObservedGeneration = secretStore.Generation
	secretStore.Status.Conditions = setStatusCondition(
		secretStore.Status.Conditions,
		declarestv1alpha1.ConditionTypeReady,
		metav1.ConditionFalse,
		reason,
		message,
	)
	secretStore.Status.Conditions = setStatusCondition(
		secretStore.Status.Conditions,
		declarestv1alpha1.ConditionTypeStalled,
		metav1.ConditionTrue,
		reason,
		message,
	)
	return r.Status().Update(ctx, secretStore)
}

func (r *SecretStoreReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&declarestv1alpha1.SecretStore{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
