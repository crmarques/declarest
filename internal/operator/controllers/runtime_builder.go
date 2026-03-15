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

package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	"github.com/crmarques/declarest/config"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type runtimeContextBuildResult struct {
	ResolvedContext       config.Context
	RepositoryLocalPath   string
	ManagedServerOpenAPI  string
	ManagedServerMetadata string
	Cleanup               func()
}

func buildRuntimeContext(
	ctx context.Context,
	reader client.Reader,
	policy *declarestv1alpha1.SyncPolicy,
	repo *declarestv1alpha1.ResourceRepository,
	managedServer *declarestv1alpha1.ManagedServer,
	secretStore *declarestv1alpha1.SecretStore,
) (runtimeContextBuildResult, error) {
	if policy == nil || repo == nil || managedServer == nil || secretStore == nil {
		return runtimeContextBuildResult{}, fmt.Errorf("build runtime context requires non-nil resources")
	}
	cleanup := &cleanupRegistry{}
	repositoryPath := strings.TrimSpace(repo.Status.LocalPath)
	if repositoryPath == "" {
		repositoryPath = resolveRepoRootPath(repo.Namespace, repo.Name)
	}
	if _, err := os.Stat(repositoryPath); err != nil {
		return runtimeContextBuildResult{}, fmt.Errorf("repository local path %q is unavailable: %w", repositoryPath, err)
	}

	cacheDir := resolveCacheRootPath(policy.Namespace, policy.Name)
	proxyConfig, err := resolveManagedServerProxyConfig(ctx, reader, policy.Namespace, managedServer.Spec.HTTP.Proxy)
	if err != nil {
		return runtimeContextBuildResult{}, err
	}
	openAPIPath := strings.TrimSpace(managedServer.Status.OpenAPICachePath)
	if openAPIPath == "" && strings.TrimSpace(managedServer.Spec.OpenAPI.URL) != "" {
		downloaded, err := downloadArtifact(ctx, managedServer.Spec.OpenAPI.URL, filepath.Join(cacheDir, "openapi"), proxyConfig)
		if err != nil {
			return runtimeContextBuildResult{}, err
		}
		openAPIPath = downloaded
	}

	metadataPath := strings.TrimSpace(managedServer.Status.MetadataCachePath)
	metadataBundle := strings.TrimSpace(managedServer.Spec.Metadata.Bundle)
	if metadataBundle == "" && metadataPath == "" && strings.TrimSpace(managedServer.Spec.Metadata.URL) != "" {
		downloaded, err := downloadArtifact(ctx, managedServer.Spec.Metadata.URL, filepath.Join(cacheDir, "metadata"), proxyConfig)
		if err != nil {
			return runtimeContextBuildResult{}, err
		}
		metadataPath = downloaded
	}

	resolvedContext := config.Context{
		Name: policy.Name,
		Repository: config.Repository{
			Filesystem: &config.FilesystemRepository{
				BaseDir: repositoryPath,
			},
		},
		ManagedServer: &config.ManagedServer{HTTP: &config.HTTPServer{}},
	}

	if err := populateManagedServerConfig(ctx, reader, policy.Namespace, managedServer, resolvedContext.ManagedServer.HTTP, openAPIPath, cacheDir, cleanup); err != nil {
		cleanup.run()
		return runtimeContextBuildResult{}, err
	}
	if err := populateSecretStoreConfig(ctx, reader, policy.Namespace, secretStore, &resolvedContext, cacheDir, cleanup); err != nil {
		cleanup.run()
		return runtimeContextBuildResult{}, err
	}
	if err := populateMetadataConfigWithBundle(metadataPath, metadataBundle, &resolvedContext); err != nil {
		cleanup.run()
		return runtimeContextBuildResult{}, err
	}

	metadataSource := metadataBundle
	if metadataSource == "" {
		metadataSource = metadataPath
	}

	return runtimeContextBuildResult{
		ResolvedContext:       resolvedContext,
		RepositoryLocalPath:   repositoryPath,
		ManagedServerOpenAPI:  openAPIPath,
		ManagedServerMetadata: metadataSource,
		Cleanup:               cleanup.run,
	}, nil
}

func readSecretValue(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	ref *corev1.SecretKeySelector,
) (string, error) {
	return readSecretValueFromClient(ctx, reader, namespace, ref)
}
