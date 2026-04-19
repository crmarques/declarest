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

package resource

import (
	"context"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	resourcedomain "github.com/crmarques/declarest/resource"
)

type editSourceResolver interface {
	ResolveLocalResource(ctx context.Context, logicalPath string) (resourcedomain.Resource, error)
}

func resolveEditSource(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	logicalPath string,
) (string, resourcedomain.Content, error) {
	normalizedPath, err := resourcedomain.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return "", resourcedomain.Content{}, err
	}

	resolvedPath, localValue, found, err := resolveEditLocalSource(ctx, deps, normalizedPath)
	if err != nil {
		return "", resourcedomain.Content{}, err
	}
	if found {
		return resolvedPath, localValue, nil
	}

	remoteReader, err := cliutil.RequireRemoteReader(deps)
	if err != nil {
		return "", resourcedomain.Content{}, err
	}

	remoteValue, err := remoteReader.GetRemote(ctx, normalizedPath)
	if err != nil {
		return "", resourcedomain.Content{}, err
	}
	return normalizedPath, remoteValue, nil
}

func resolveEditLocalSource(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	normalizedPath string,
) (string, resourcedomain.Content, bool, error) {
	if resolver, ok := deps.Orchestrator.(editSourceResolver); ok {
		item, err := resolver.ResolveLocalResource(ctx, normalizedPath)
		if err == nil {
			return item.LogicalPath, resourcedomain.Content{
				Value:      item.Payload,
				Descriptor: item.PayloadDescriptor,
			}, true, nil
		}
		if faults.IsCategory(err, faults.NotFoundError) {
			return "", resourcedomain.Content{}, false, nil
		}
		return "", resourcedomain.Content{}, false, err
	}

	if deps.Services == nil || deps.Services.RepositoryStore() == nil {
		return "", resourcedomain.Content{}, false, nil
	}

	value, err := deps.Services.RepositoryStore().Get(ctx, normalizedPath)
	if err == nil {
		return normalizedPath, value, true, nil
	}
	if faults.IsCategory(err, faults.NotFoundError) {
		return "", resourcedomain.Content{}, false, nil
	}
	return "", resourcedomain.Content{}, false, err
}
