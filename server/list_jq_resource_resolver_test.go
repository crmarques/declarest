package server

import (
	"context"
	"strings"
	"testing"

	"github.com/crmarques/declarest/resource"
)

func TestResolveListJQResourceWithoutResolver(t *testing.T) {
	t.Parallel()

	value, resolved, err := ResolveListJQResource(context.Background(), "/customers/acme")
	if err != nil {
		t.Fatalf("ResolveListJQResource returned error: %v", err)
	}
	if resolved {
		t.Fatal("expected resolved=false when resolver is not attached")
	}
	if value != nil {
		t.Fatalf("expected nil value when unresolved, got %#v", value)
	}
}

func TestResolveListJQResourceCachesByPath(t *testing.T) {
	t.Parallel()

	var calls int
	ctx := WithListJQResourceResolver(
		context.Background(),
		func(_ context.Context, logicalPath string) (resource.Value, error) {
			calls++
			if logicalPath != "/customers/acme" {
				t.Fatalf("unexpected logical path %q", logicalPath)
			}
			return map[string]any{"id": "1"}, nil
		},
	)

	firstValue, firstResolved, firstErr := ResolveListJQResource(ctx, "/customers/acme/")
	if firstErr != nil {
		t.Fatalf("first resolve returned error: %v", firstErr)
	}
	if !firstResolved {
		t.Fatal("expected first resolve to be resolved")
	}
	secondValue, secondResolved, secondErr := ResolveListJQResource(ctx, "/customers/acme")
	if secondErr != nil {
		t.Fatalf("second resolve returned error: %v", secondErr)
	}
	if !secondResolved {
		t.Fatal("expected second resolve to be resolved")
	}
	if calls != 1 {
		t.Fatalf("expected one resolver call due to cache, got %d", calls)
	}

	firstMap, firstOK := firstValue.(map[string]any)
	secondMap, secondOK := secondValue.(map[string]any)
	if !firstOK || !secondOK {
		t.Fatalf("expected map values, got %T and %T", firstValue, secondValue)
	}
	if firstMap["id"] != "1" || secondMap["id"] != "1" {
		t.Fatalf("expected cached payload id=1, got %#v and %#v", firstMap, secondMap)
	}
}

func TestResolveListJQResourceDetectsCycles(t *testing.T) {
	t.Parallel()

	var resolver ListJQResourceResolver
	resolver = func(ctx context.Context, logicalPath string) (resource.Value, error) {
		switch logicalPath {
		case "/one":
			_, _, err := ResolveListJQResource(ctx, "/two")
			return nil, err
		case "/two":
			_, _, err := ResolveListJQResource(ctx, "/one")
			return nil, err
		default:
			return map[string]any{"id": logicalPath}, nil
		}
	}

	ctx := WithListJQResourceResolver(context.Background(), resolver)

	_, resolved, err := ResolveListJQResource(ctx, "/one")
	if !resolved {
		t.Fatal("expected cycle resolution attempt to be marked as resolved")
	}
	if err == nil {
		t.Fatal("expected cycle error")
	}
	if !strings.Contains(err.Error(), "cyclic dependency") {
		t.Fatalf("expected cycle error message, got %q", err.Error())
	}
}
