package server

import (
	"context"
	"fmt"
	"sync"

	"github.com/crmarques/declarest/resource"
)

// ListJQResourceResolver resolves logical-path resources for metadata list jq resource() calls.
type ListJQResourceResolver func(ctx context.Context, logicalPath string) (resource.Value, error)

type listJQResolverContextKey struct{}
type listJQResolverStateContextKey struct{}

type listJQResolverState struct {
	mu       sync.Mutex
	cache    map[string]resource.Value
	inFlight map[string]struct{}
}

// WithListJQResourceResolver attaches a logical-path resolver to the context.
func WithListJQResourceResolver(ctx context.Context, resolver ListJQResourceResolver) context.Context {
	if resolver == nil {
		return ctx
	}
	if _, exists := listJQResourceResolverFromContext(ctx); exists {
		return ctx
	}

	state := &listJQResolverState{
		cache:    map[string]resource.Value{},
		inFlight: map[string]struct{}{},
	}

	ctx = context.WithValue(ctx, listJQResolverContextKey{}, resolver)
	ctx = context.WithValue(ctx, listJQResolverStateContextKey{}, state)
	return ctx
}

// ResolveListJQResource resolves logical-path values for metadata list jq resource() calls.
func ResolveListJQResource(ctx context.Context, logicalPath string) (resource.Value, bool, error) {
	resolver, exists := listJQResourceResolverFromContext(ctx)
	if !exists || resolver == nil {
		return nil, false, nil
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, true, err
	}

	state := listJQResolverStateFromContext(ctx)
	if state != nil {
		if cached, ok := stateCachedValue(state, normalizedPath); ok {
			return cached, true, nil
		}
		if !stateMarkInFlight(state, normalizedPath) {
			return nil, true, fmt.Errorf("resource() path %q creates a cyclic dependency", normalizedPath)
		}
		defer stateUnmarkInFlight(state, normalizedPath)
	}

	value, resolveErr := resolver(ctx, normalizedPath)
	if resolveErr != nil {
		return nil, true, resolveErr
	}

	if state != nil {
		stateStoreValue(state, normalizedPath, value)
	}
	return value, true, nil
}

func listJQResourceResolverFromContext(ctx context.Context) (ListJQResourceResolver, bool) {
	if ctx == nil {
		return nil, false
	}
	value := ctx.Value(listJQResolverContextKey{})
	resolver, ok := value.(ListJQResourceResolver)
	return resolver, ok
}

func listJQResolverStateFromContext(ctx context.Context) *listJQResolverState {
	if ctx == nil {
		return nil
	}
	value := ctx.Value(listJQResolverStateContextKey{})
	state, _ := value.(*listJQResolverState)
	return state
}

func stateCachedValue(state *listJQResolverState, logicalPath string) (resource.Value, bool) {
	state.mu.Lock()
	defer state.mu.Unlock()
	value, exists := state.cache[logicalPath]
	return value, exists
}

func stateStoreValue(state *listJQResolverState, logicalPath string, value resource.Value) {
	state.mu.Lock()
	defer state.mu.Unlock()
	state.cache[logicalPath] = value
}

func stateMarkInFlight(state *listJQResolverState, logicalPath string) bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	if _, exists := state.inFlight[logicalPath]; exists {
		return false
	}
	state.inFlight[logicalPath] = struct{}{}
	return true
}

func stateUnmarkInFlight(state *listJQResolverState, logicalPath string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	delete(state.inFlight, logicalPath)
}
