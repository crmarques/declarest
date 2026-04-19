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

package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/envref"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-resourcerepository,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=resourcerepositories,verbs=create;update;delete,versions=v1alpha1,name=vresourcerepository-v1alpha1.declarest.io,admissionReviewVersions=v1
func (r *ResourceRepository) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, r).
		WithValidator(&resourceRepositoryValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-managedservice,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=managedservices,verbs=create;update;delete,versions=v1alpha1,name=vmanagedservice-v1alpha1.declarest.io,admissionReviewVersions=v1
func (m *ManagedService) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, m).
		WithValidator(&managedServiceValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-secretstore,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=secretstores,verbs=create;update;delete,versions=v1alpha1,name=vsecretstore-v1alpha1.declarest.io,admissionReviewVersions=v1
func (s *SecretStore) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, s).
		WithValidator(&secretStoreValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-syncpolicy,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=syncpolicies,verbs=create;update,versions=v1alpha1,name=vsyncpolicy-v1alpha1.declarest.io,admissionReviewVersions=v1
func (s *SyncPolicy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, s).
		WithValidator(&syncPolicyValidator{Client: mgr.GetClient()}).
		Complete()
}

// --- ResourceRepository Validator ---

type resourceRepositoryValidator struct {
	Client client.Reader
}

func (v *resourceRepositoryValidator) ValidateCreate(_ context.Context, obj *ResourceRepository) (admission.Warnings, error) {
	candidate := obj.DeepCopy()
	envref.ExpandExactEnvPlaceholdersInPlace(&candidate.Spec)
	candidate.Default()
	return nil, candidate.ValidateSpec()
}

func (v *resourceRepositoryValidator) ValidateUpdate(ctx context.Context, _ *ResourceRepository, newObj *ResourceRepository) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, newObj)
}

func (v *resourceRepositoryValidator) ValidateDelete(ctx context.Context, obj *ResourceRepository) (admission.Warnings, error) {
	repo := obj
	return checkDependencyRef(ctx, v.Client, repo.Namespace, "ResourceRepository", repo.Name, func(sp *SyncPolicy) string {
		return sp.Spec.ResourceRepositoryRef.Name
	})
}

// --- ManagedService Validator ---

type managedServiceValidator struct {
	Client client.Reader
}

func (v *managedServiceValidator) ValidateCreate(_ context.Context, obj *ManagedService) (admission.Warnings, error) {
	candidate := obj.DeepCopy()
	envref.ExpandExactEnvPlaceholdersInPlace(&candidate.Spec)
	candidate.Default()
	return nil, candidate.ValidateSpec()
}

func (v *managedServiceValidator) ValidateUpdate(ctx context.Context, _ *ManagedService, newObj *ManagedService) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, newObj)
}

func (v *managedServiceValidator) ValidateDelete(ctx context.Context, obj *ManagedService) (admission.Warnings, error) {
	ms := obj
	return checkDependencyRef(ctx, v.Client, ms.Namespace, "ManagedService", ms.Name, func(sp *SyncPolicy) string {
		return sp.Spec.ManagedServiceRef.Name
	})
}

// --- SecretStore Validator ---

type secretStoreValidator struct {
	Client client.Reader
}

func (v *secretStoreValidator) ValidateCreate(_ context.Context, obj *SecretStore) (admission.Warnings, error) {
	candidate := obj.DeepCopy()
	envref.ExpandExactEnvPlaceholdersInPlace(&candidate.Spec)
	return nil, candidate.ValidateSpec()
}

func (v *secretStoreValidator) ValidateUpdate(ctx context.Context, _ *SecretStore, newObj *SecretStore) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, newObj)
}

func (v *secretStoreValidator) ValidateDelete(ctx context.Context, obj *SecretStore) (admission.Warnings, error) {
	ss := obj
	return checkDependencyRef(ctx, v.Client, ss.Namespace, "SecretStore", ss.Name, func(sp *SyncPolicy) string {
		return sp.Spec.SecretStoreRef.Name
	})
}

// --- SyncPolicy Validator ---

type syncPolicyValidator struct {
	Client client.Reader
}

func (v *syncPolicyValidator) ValidateCreate(ctx context.Context, obj *SyncPolicy) (admission.Warnings, error) {
	candidate := obj.DeepCopy()
	envref.ExpandExactEnvPlaceholdersInPlace(&candidate.Spec)
	candidate.Default()
	if err := candidate.ValidateSpec(); err != nil {
		return nil, err
	}
	return v.validateNoOverlap(ctx, candidate)
}

func (v *syncPolicyValidator) ValidateUpdate(ctx context.Context, _ *SyncPolicy, newObj *SyncPolicy) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, newObj)
}

func (v *syncPolicyValidator) ValidateDelete(context.Context, *SyncPolicy) (admission.Warnings, error) {
	return nil, nil
}

func (v *syncPolicyValidator) validateNoOverlap(ctx context.Context, syncPolicy *SyncPolicy) (admission.Warnings, error) {
	policies := &SyncPolicyList{}
	if err := v.Client.List(ctx, policies, client.InNamespace(syncPolicy.Namespace)); err != nil {
		// If we cannot list, allow the operation but warn.
		return admission.Warnings{"unable to verify SyncPolicy overlap, proceeding"}, nil
	}
	for idx := range policies.Items {
		item := &policies.Items[idx]
		expandedItem := item.DeepCopy()
		envref.ExpandExactEnvPlaceholdersInPlace(&expandedItem.Spec)
		if item.Name == syncPolicy.Name {
			continue
		}
		if item.DeletionTimestamp != nil {
			continue
		}
		if HasPathOverlap(expandedItem.Spec.Source.Path, syncPolicy.Spec.Source.Path) {
			return nil, fmt.Errorf(
				"sync policy scope overlaps with %s/%s (%q)",
				item.Namespace,
				item.Name,
				NormalizeOverlapPath(expandedItem.Spec.Source.Path),
			)
		}
	}
	return nil, nil
}

// --- Shared helpers ---

// checkDependencyRef checks whether any SyncPolicy in the given namespace
// references the resource being deleted. Returns an error to block deletion
// if references exist.
func checkDependencyRef(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	kind string,
	name string,
	refExtractor func(*SyncPolicy) string,
) (admission.Warnings, error) {
	policies := &SyncPolicyList{}
	if err := reader.List(ctx, policies, client.InNamespace(namespace)); err != nil {
		// If we cannot verify, allow deletion but warn.
		return admission.Warnings{fmt.Sprintf("unable to verify SyncPolicy references for %s %q, deletion allowed", kind, name)}, nil
	}
	var refs []string
	for idx := range policies.Items {
		item := &policies.Items[idx]
		expandedItem := item.DeepCopy()
		envref.ExpandExactEnvPlaceholdersInPlace(&expandedItem.Spec)
		if item.DeletionTimestamp != nil {
			continue
		}
		if strings.TrimSpace(refExtractor(expandedItem)) == name {
			refs = append(refs, item.Name)
		}
	}
	if len(refs) > 0 {
		return nil, fmt.Errorf(
			"cannot delete %s %q: referenced by SyncPolicy %s",
			kind,
			name,
			strings.Join(refs, ", "),
		)
	}
	return nil, nil
}
