package resource

import (
	"context"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	resourcedomain "github.com/crmarques/declarest/resource"
)

type localResourceResolver interface {
	ResolveLocalResource(ctx context.Context, logicalPath string) (resourcedomain.Resource, error)
}

func resolveEditSource(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	logicalPath string,
) (string, resourcedomain.Value, error) {
	normalizedPath, err := resourcedomain.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return "", nil, err
	}

	resolvedPath, localValue, found, err := resolveEditLocalSource(ctx, deps, normalizedPath)
	if err != nil {
		return "", nil, err
	}
	if found {
		return resolvedPath, localValue, nil
	}

	remoteReader, err := cliutil.RequireRemoteReader(deps)
	if err != nil {
		return "", nil, err
	}

	remoteValue, err := remoteReader.GetRemote(ctx, normalizedPath)
	if err != nil {
		return "", nil, err
	}
	return normalizedPath, remoteValue, nil
}

func resolveEditLocalSource(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	normalizedPath string,
) (string, resourcedomain.Value, bool, error) {
	if resolver, ok := deps.Orchestrator.(localResourceResolver); ok {
		item, err := resolver.ResolveLocalResource(ctx, normalizedPath)
		if err == nil {
			return item.LogicalPath, item.Payload, true, nil
		}
		if faults.IsCategory(err, faults.NotFoundError) {
			return "", nil, false, nil
		}
		return "", nil, false, err
	}

	if deps.ResourceStore == nil {
		return "", nil, false, nil
	}

	value, err := deps.ResourceStore.Get(ctx, normalizedPath)
	if err == nil {
		return normalizedPath, value, true, nil
	}
	if faults.IsCategory(err, faults.NotFoundError) {
		return "", nil, false, nil
	}
	return "", nil, false, err
}
