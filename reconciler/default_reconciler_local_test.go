package reconciler

import (
	"reflect"
	"strings"
	"testing"

	"github.com/crmarques/declarest/resource"
)

type stubRecordProvider struct {
	record resource.ResourceRecord
	err    error
}

func (s *stubRecordProvider) GetResourceRecord(string) (resource.ResourceRecord, error) {
	if s.err != nil {
		return resource.ResourceRecord{}, s.err
	}
	return s.record, nil
}

func (s *stubRecordProvider) GetMergedMetadata(string) (resource.ResourceMetadata, error) {
	if s.err != nil {
		return resource.ResourceMetadata{}, s.err
	}
	return s.record.Meta, nil
}

func TestLocalChildResourcesMatchesWildcardSegments(t *testing.T) {
	repo := &stubRepoManager{
		paths: []string{
			"/realms/realm-1/clients/client-1",
			"/realms/realm-2/clients/client-2",
			"/realms/realm-1/roles/role-1",
			"/realms/realm-1/clients",
		},
	}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}

	got, err := recon.localChildResources("/realms/_/clients")
	if err != nil {
		t.Fatalf("localChildResources: %v", err)
	}

	want := []string{
		"/realms/realm-1/clients/client-1",
		"/realms/realm-2/clients/client-2",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestUpdateLocalResourceForMetadataKeepsPathWhenAliasMatches(t *testing.T) {
	repo := &stubRepoManager{}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}
	recon.ResourceRecordProvider = &stubRecordProvider{
		record: resource.ResourceRecord{
			Path: "/items/foo",
			Meta: resource.ResourceMetadata{
				ResourceInfo: &resource.ResourceInfoMetadata{
					AliasFromAttribute: "name",
				},
			},
		},
	}

	res := mustResource(t, map[string]any{"name": "foo"})
	updatedPath, moved, err := recon.updateLocalResourceForMetadata("/items/foo", res)
	if err != nil {
		t.Fatalf("updateLocalResourceForMetadata: %v", err)
	}
	if moved {
		t.Fatal("expected resource to remain in place")
	}
	if updatedPath != "/items/foo" {
		t.Fatalf("expected updated path /items/foo, got %q", updatedPath)
	}
	if len(repo.applyCalls) != 1 || repo.applyCalls[0] != "/items/foo" {
		t.Fatalf("expected ApplyResource to target /items/foo, got %v", repo.applyCalls)
	}
}

func TestUpdateLocalResourceForMetadataErrorsWhenMoverMissing(t *testing.T) {
	repo := &stubRepoManager{}
	recon := &DefaultReconciler{ResourceRepositoryManager: repo}
	recon.ResourceRecordProvider = &stubRecordProvider{
		record: resource.ResourceRecord{
			Path: "/items/legacy",
			Meta: resource.ResourceMetadata{
				ResourceInfo: &resource.ResourceInfoMetadata{
					AliasFromAttribute: "name",
				},
			},
		},
	}

	res := mustResource(t, map[string]any{"name": "pretty"})
	_, _, err := recon.updateLocalResourceForMetadata("/items/legacy", res)
	if err == nil {
		t.Fatal("expected updateLocalResourceForMetadata to fail without mover support")
	}
	if !strings.Contains(err.Error(), "does not support moving resources") {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.applyCalls) != 0 {
		t.Fatalf("expected ApplyResource not to run, got %v", repo.applyCalls)
	}
}
