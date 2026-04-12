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

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	k8sevents "k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type RepositoryWebhookReconciler struct {
	Client                  client.Client
	Scheme                  *runtime.Scheme
	Recorder                k8sevents.EventRecorder
	MaxConcurrentReconciles int
}

func (r *RepositoryWebhookReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	rwh := &declarestv1alpha1.RepositoryWebhook{}
	if err := r.Client.Get(ctx, req.NamespacedName, rwh); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Verify the referenced ResourceRepository exists.
	repo := &declarestv1alpha1.ResourceRepository{}
	repoKey := types.NamespacedName{
		Namespace: rwh.Namespace,
		Name:      rwh.Spec.RepositoryRef.Name,
	}
	repoExists := true
	if err := r.Client.Get(ctx, repoKey, repo); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		repoExists = false
	}

	// Build the webhook path.
	webhookPath := fmt.Sprintf("/hooks/v1/repositorywebhooks/%s/%s", rwh.Namespace, rwh.Name)

	// Update status.
	newConditions := rwh.Status.Conditions
	if rwh.Spec.Suspend {
		newConditions = declarestv1alpha1.SetCondition(newConditions, metav1.Condition{
			Type:               declarestv1alpha1.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             "Suspended",
			Message:            "RepositoryWebhook is suspended",
			ObservedGeneration: rwh.Generation,
			LastTransitionTime: metav1.Now(),
		})
	} else if !repoExists {
		newConditions = declarestv1alpha1.SetCondition(newConditions, metav1.Condition{
			Type:               declarestv1alpha1.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             "RepositoryNotFound",
			Message:            fmt.Sprintf("ResourceRepository %q not found", rwh.Spec.RepositoryRef.Name),
			ObservedGeneration: rwh.Generation,
			LastTransitionTime: metav1.Now(),
		})
	} else {
		newConditions = declarestv1alpha1.SetCondition(newConditions, metav1.Condition{
			Type:               declarestv1alpha1.ConditionTypeReady,
			Status:             metav1.ConditionTrue,
			Reason:             "Ready",
			Message:            "RepositoryWebhook is ready to receive events",
			ObservedGeneration: rwh.Generation,
			LastTransitionTime: metav1.Now(),
		})
	}

	rwh.Status.ObservedGeneration = rwh.Generation
	rwh.Status.WebhookPath = webhookPath
	rwh.Status.Conditions = newConditions

	if err := r.Client.Status().Update(ctx, rwh); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{Requeue: true}, nil
		}
		logger.Error(err, "failed to update RepositoryWebhook status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *RepositoryWebhookReconciler) SetupWithManager(mgr ctrl.Manager) error {
	bld := ctrl.NewControllerManagedBy(mgr).
		For(&declarestv1alpha1.RepositoryWebhook{})
	if r.MaxConcurrentReconciles > 0 {
		bld = bld.WithOptions(controller.Options{MaxConcurrentReconciles: r.MaxConcurrentReconciles})
	}
	return bld.Complete(r)
}
