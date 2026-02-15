package reconciler

import (
	"context"
	"errors"
	"testing"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func TestDefaultReconcilerDelegatesRepositoryMethods(t *testing.T) {
	t.Parallel()

	fakeRepo := &fakeRepositoryManager{
		getValue: resource.Value(map[string]any{"id": int64(1)}),
		listValue: []resource.Resource{
			{LogicalPath: "/customers/acme"},
		},
		statusValue: repository.SyncReport{State: repository.SyncStateNoRemote},
	}

	reconciler := &DefaultReconciler{
		RepositoryManager: fakeRepo,
	}

	value, err := reconciler.Get(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	if value == nil {
		t.Fatal("expected non-nil value")
	}

	if err := reconciler.Save(context.Background(), "/customers/acme", map[string]any{"x": 1}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	if err := reconciler.Delete(context.Background(), "/customers", DeletePolicy{Recursive: true}); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	items, err := reconciler.ListLocal(context.Background(), "/customers", ListPolicy{Recursive: true})
	if err != nil {
		t.Fatalf("ListLocal returned error: %v", err)
	}
	if len(items) != 1 || items[0].LogicalPath != "/customers/acme" {
		t.Fatalf("unexpected list output: %#v", items)
	}

	status, err := reconciler.RepoStatus(context.Background())
	if err != nil {
		t.Fatalf("RepoStatus returned error: %v", err)
	}
	if status.State != repository.SyncStateNoRemote {
		t.Fatalf("unexpected status state: %q", status.State)
	}

	if !fakeRepo.deletePolicy.Recursive {
		t.Fatal("expected delete policy recursion to be mapped")
	}
	if !fakeRepo.listPolicy.Recursive {
		t.Fatal("expected list policy recursion to be mapped")
	}
}

func TestDefaultReconcilerRequiresRepositoryManager(t *testing.T) {
	t.Parallel()

	reconciler := &DefaultReconciler{}

	_, err := reconciler.RepoStatus(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	assertTypedCategory(t, err, faults.ValidationError)
}

type fakeRepositoryManager struct {
	getValue    resource.Value
	listValue   []resource.Resource
	statusValue repository.SyncReport

	deletePolicy repository.DeletePolicy
	listPolicy   repository.ListPolicy
}

func (f *fakeRepositoryManager) Save(context.Context, string, resource.Value) error { return nil }
func (f *fakeRepositoryManager) Get(context.Context, string) (resource.Value, error) {
	return f.getValue, nil
}
func (f *fakeRepositoryManager) Delete(_ context.Context, _ string, policy repository.DeletePolicy) error {
	f.deletePolicy = policy
	return nil
}
func (f *fakeRepositoryManager) List(_ context.Context, _ string, policy repository.ListPolicy) ([]resource.Resource, error) {
	f.listPolicy = policy
	return f.listValue, nil
}
func (f *fakeRepositoryManager) Exists(context.Context, string) (bool, error) { return false, nil }
func (f *fakeRepositoryManager) Move(context.Context, string, string) error   { return nil }
func (f *fakeRepositoryManager) Init(context.Context) error                   { return nil }
func (f *fakeRepositoryManager) Refresh(context.Context) error                { return nil }
func (f *fakeRepositoryManager) Reset(context.Context, repository.ResetPolicy) error {
	return nil
}
func (f *fakeRepositoryManager) Check(context.Context) error { return nil }
func (f *fakeRepositoryManager) Push(context.Context, repository.PushPolicy) error {
	return nil
}
func (f *fakeRepositoryManager) SyncStatus(context.Context) (repository.SyncReport, error) {
	return f.statusValue, nil
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != category {
		t.Fatalf("expected %q category, got %q", category, typedErr.Category)
	}
}
