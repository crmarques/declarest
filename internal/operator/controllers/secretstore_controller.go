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

// SecretStoreReconciler reconciles SecretStore resources.
type SecretStoreReconciler struct {
	client.Client
	Scheme                  *runtime.Scheme
	Recorder                k8sevents.EventRecorder
	MaxConcurrentReconciles int
}

func (r *SecretStoreReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("secretStore", req.String(), "reconcile_id", uuidString())
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
	bld := ctrl.NewControllerManagedBy(mgr).
		For(&declarestv1alpha1.SecretStore{}, builder.WithPredicates(predicate.GenerationChangedPredicate{}))
	if r.MaxConcurrentReconciles > 0 {
		bld = bld.WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles})
	}
	return bld.Complete(r)
}
