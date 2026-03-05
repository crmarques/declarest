package v1alpha1

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-resourcerepository,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=resourcerepositories,verbs=create;update;delete,versions=v1alpha1,name=vresourcerepository-v1alpha1.declarest.io,admissionReviewVersions=v1
func (r *ResourceRepository) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(&resourceRepositoryValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-managedserver,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=managedservers,verbs=create;update;delete,versions=v1alpha1,name=vmanagedserver-v1alpha1.declarest.io,admissionReviewVersions=v1
func (m *ManagedServer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		WithValidator(&managedServerValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-secretstore,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=secretstores,verbs=create;update;delete,versions=v1alpha1,name=vsecretstore-v1alpha1.declarest.io,admissionReviewVersions=v1
func (s *SecretStore) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		WithValidator(&secretStoreValidator{Client: mgr.GetClient()}).
		Complete()
}

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-syncpolicy,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=syncpolicies,verbs=create;update,versions=v1alpha1,name=vsyncpolicy-v1alpha1.declarest.io,admissionReviewVersions=v1
func (s *SyncPolicy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		WithValidator(&syncPolicyValidator{Client: mgr.GetClient()}).
		Complete()
}

// --- ResourceRepository Validator ---

type resourceRepositoryValidator struct {
	Client client.Reader
}

func (v *resourceRepositoryValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	value, ok := obj.(*ResourceRepository)
	if !ok {
		return nil, fmt.Errorf("expected ResourceRepository, got %T", obj)
	}
	candidate := value.DeepCopy()
	candidate.Default()
	return nil, candidate.ValidateSpec()
}

func (v *resourceRepositoryValidator) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, newObj)
}

func (v *resourceRepositoryValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	repo, ok := obj.(*ResourceRepository)
	if !ok {
		return nil, fmt.Errorf("expected ResourceRepository, got %T", obj)
	}
	return checkDependencyRef(ctx, v.Client, repo.Namespace, "ResourceRepository", repo.Name, func(sp *SyncPolicy) string {
		return sp.Spec.ResourceRepositoryRef.Name
	})
}

// --- ManagedServer Validator ---

type managedServerValidator struct {
	Client client.Reader
}

func (v *managedServerValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	value, ok := obj.(*ManagedServer)
	if !ok {
		return nil, fmt.Errorf("expected ManagedServer, got %T", obj)
	}
	candidate := value.DeepCopy()
	candidate.Default()
	return nil, candidate.ValidateSpec()
}

func (v *managedServerValidator) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, newObj)
}

func (v *managedServerValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	ms, ok := obj.(*ManagedServer)
	if !ok {
		return nil, fmt.Errorf("expected ManagedServer, got %T", obj)
	}
	return checkDependencyRef(ctx, v.Client, ms.Namespace, "ManagedServer", ms.Name, func(sp *SyncPolicy) string {
		return sp.Spec.ManagedServerRef.Name
	})
}

// --- SecretStore Validator ---

type secretStoreValidator struct {
	Client client.Reader
}

func (v *secretStoreValidator) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	value, ok := obj.(*SecretStore)
	if !ok {
		return nil, fmt.Errorf("expected SecretStore, got %T", obj)
	}
	candidate := value.DeepCopy()
	return nil, candidate.ValidateSpec()
}

func (v *secretStoreValidator) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, newObj)
}

func (v *secretStoreValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	ss, ok := obj.(*SecretStore)
	if !ok {
		return nil, fmt.Errorf("expected SecretStore, got %T", obj)
	}
	return checkDependencyRef(ctx, v.Client, ss.Namespace, "SecretStore", ss.Name, func(sp *SyncPolicy) string {
		return sp.Spec.SecretStoreRef.Name
	})
}

// --- SyncPolicy Validator ---

type syncPolicyValidator struct {
	Client client.Reader
}

func (v *syncPolicyValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	value, ok := obj.(*SyncPolicy)
	if !ok {
		return nil, fmt.Errorf("expected SyncPolicy, got %T", obj)
	}
	candidate := value.DeepCopy()
	candidate.Default()
	if err := candidate.ValidateSpec(); err != nil {
		return nil, err
	}
	return v.validateNoOverlap(ctx, candidate)
}

func (v *syncPolicyValidator) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	return v.ValidateCreate(ctx, newObj)
}

func (v *syncPolicyValidator) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
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
		if item.Name == syncPolicy.Name {
			continue
		}
		if item.DeletionTimestamp != nil {
			continue
		}
		if HasPathOverlap(item.Spec.Source.Path, syncPolicy.Spec.Source.Path) {
			return nil, fmt.Errorf(
				"sync policy scope overlaps with %s/%s (%q)",
				item.Namespace,
				item.Name,
				NormalizeOverlapPath(item.Spec.Source.Path),
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
		if item.DeletionTimestamp != nil {
			continue
		}
		if strings.TrimSpace(refExtractor(item)) == name {
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
