package v1alpha1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-resourcerepository,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=resourcerepositories,verbs=create;update,versions=v1alpha1,name=vresourcerepository-v1alpha1.declarest.io,admissionReviewVersions=v1
func (r *ResourceRepository) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		WithValidator(r).
		Complete()
}

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-managedserver,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=managedservers,verbs=create;update,versions=v1alpha1,name=vmanagedserver-v1alpha1.declarest.io,admissionReviewVersions=v1
func (m *ManagedServer) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(m).
		WithValidator(m).
		Complete()
}

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-secretstore,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=secretstores,verbs=create;update,versions=v1alpha1,name=vsecretstore-v1alpha1.declarest.io,admissionReviewVersions=v1
func (s *SecretStore) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		WithValidator(s).
		Complete()
}

// +kubebuilder:webhook:path=/validate-declarest-io-v1alpha1-syncpolicy,mutating=false,failurePolicy=Fail,sideEffects=None,groups=declarest.io,resources=syncpolicies,verbs=create;update,versions=v1alpha1,name=vsyncpolicy-v1alpha1.declarest.io,admissionReviewVersions=v1
func (s *SyncPolicy) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(s).
		WithValidator(s).
		Complete()
}

func (r *ResourceRepository) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	value, ok := obj.(*ResourceRepository)
	if !ok {
		return nil, fmt.Errorf("expected ResourceRepository, got %T", obj)
	}
	candidate := value.DeepCopy()
	candidate.Default()
	return nil, candidate.ValidateSpec()
}

func (r *ResourceRepository) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	return r.ValidateCreate(ctx, newObj)
}

func (r *ResourceRepository) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (m *ManagedServer) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	value, ok := obj.(*ManagedServer)
	if !ok {
		return nil, fmt.Errorf("expected ManagedServer, got %T", obj)
	}
	candidate := value.DeepCopy()
	candidate.Default()
	return nil, candidate.ValidateSpec()
}

func (m *ManagedServer) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	return m.ValidateCreate(ctx, newObj)
}

func (m *ManagedServer) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (s *SecretStore) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	value, ok := obj.(*SecretStore)
	if !ok {
		return nil, fmt.Errorf("expected SecretStore, got %T", obj)
	}
	candidate := value.DeepCopy()
	return nil, candidate.ValidateSpec()
}

func (s *SecretStore) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	return s.ValidateCreate(ctx, newObj)
}

func (s *SecretStore) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (s *SyncPolicy) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	value, ok := obj.(*SyncPolicy)
	if !ok {
		return nil, fmt.Errorf("expected SyncPolicy, got %T", obj)
	}
	candidate := value.DeepCopy()
	candidate.Default()
	return nil, candidate.ValidateSpec()
}

func (s *SyncPolicy) ValidateUpdate(ctx context.Context, _ runtime.Object, newObj runtime.Object) (admission.Warnings, error) {
	return s.ValidateCreate(ctx, newObj)
}

func (s *SyncPolicy) ValidateDelete(context.Context, runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
