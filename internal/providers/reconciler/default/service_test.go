package defaultreconciler

import (
	"context"
	"errors"
	"testing"

	"github.com/crmarques/declarest/faults"
	reconcilerdomain "github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
)

func TestDefaultResourceReconcilerDelegatesRepoMethods(t *testing.T) {
	t.Parallel()

	fakeRepo := &fakeRepository{
		getValue: resource.Value(map[string]any{"id": int64(1)}),
		listValue: []resource.Resource{
			{LogicalPath: "/customers/acme"},
		},
		statusValue: repository.SyncReport{State: repository.SyncStateNoRemote},
	}

	reconciler := NewResourceReconciler(func(context.Context) (repository.ResourceRepository, error) {
		return fakeRepo, nil
	})

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

	if err := reconciler.Delete(context.Background(), "/customers", reconcilerdomain.DeletePolicy{Recursive: true}); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	items, err := reconciler.ListLocal(context.Background(), "/customers", reconcilerdomain.ListPolicy{Recursive: true})
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

func TestDefaultResourceReconcilerResolverFailure(t *testing.T) {
	t.Parallel()

	expected := faults.NewTypedError(faults.NotFoundError, "context missing", nil)
	reconciler := NewResourceReconciler(func(context.Context) (repository.ResourceRepository, error) {
		return nil, expected
	})

	_, err := reconciler.RepoStatus(context.Background())
	if !errors.Is(err, expected) {
		t.Fatalf("expected propagated resolver error, got %v", err)
	}
}

type fakeRepository struct {
	getValue    resource.Value
	listValue   []resource.Resource
	statusValue repository.SyncReport

	deletePolicy repository.DeletePolicy
	listPolicy   repository.ListPolicy
}

func (f *fakeRepository) Save(context.Context, string, resource.Value) error { return nil }
func (f *fakeRepository) Get(context.Context, string) (resource.Value, error) {
	return f.getValue, nil
}
func (f *fakeRepository) Delete(_ context.Context, _ string, policy repository.DeletePolicy) error {
	f.deletePolicy = policy
	return nil
}
func (f *fakeRepository) List(_ context.Context, _ string, policy repository.ListPolicy) ([]resource.Resource, error) {
	f.listPolicy = policy
	return f.listValue, nil
}
func (f *fakeRepository) Exists(context.Context, string) (bool, error) { return false, nil }
func (f *fakeRepository) Move(context.Context, string, string) error   { return nil }
func (f *fakeRepository) Init(context.Context) error                   { return nil }
func (f *fakeRepository) Refresh(context.Context) error                { return nil }
func (f *fakeRepository) Reset(context.Context, repository.ResetPolicy) error {
	return nil
}
func (f *fakeRepository) Check(context.Context) error { return nil }
func (f *fakeRepository) Push(context.Context, repository.PushPolicy) error {
	return nil
}
func (f *fakeRepository) SyncStatus(context.Context) (repository.SyncReport, error) {
	return f.statusValue, nil
}
