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
	openAPIPath := strings.TrimSpace(managedServer.Status.OpenAPICachePath)
	if openAPIPath == "" && strings.TrimSpace(managedServer.Spec.OpenAPI.URL) != "" {
		downloaded, err := downloadArtifact(ctx, managedServer.Spec.OpenAPI.URL, filepath.Join(cacheDir, "openapi"))
		if err != nil {
			return runtimeContextBuildResult{}, err
		}
		openAPIPath = downloaded
	}

	metadataPath := strings.TrimSpace(managedServer.Status.MetadataCachePath)
	metadataBundle := strings.TrimSpace(managedServer.Spec.Metadata.Bundle)
	if metadataBundle == "" && metadataPath == "" && strings.TrimSpace(managedServer.Spec.Metadata.URL) != "" {
		downloaded, err := downloadArtifact(ctx, managedServer.Spec.Metadata.URL, filepath.Join(cacheDir, "metadata"))
		if err != nil {
			return runtimeContextBuildResult{}, err
		}
		metadataPath = downloaded
	}

	resolvedContext := config.Context{
		Name: policy.Name,
		Repository: config.Repository{
			ResourceFormat: normalizeResourceFormat(repo.Spec.ResourceFormat),
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

func normalizeResourceFormat(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if value == "yaml" {
		return config.ResourceFormatYAML
	}
	return config.ResourceFormatJSON
}

func readSecretValue(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	ref *corev1.SecretKeySelector,
) (string, error) {
	return readSecretValueFromClient(ctx, reader, namespace, ref)
}
