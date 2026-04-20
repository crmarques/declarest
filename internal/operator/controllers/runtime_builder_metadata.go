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
	"strings"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	"github.com/crmarques/declarest/config"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func populateMetadataConfigWithBundle(metadataPath string, metadataBundle string, resolvedContext *config.Context) error {
	metadataBundle = strings.TrimSpace(metadataBundle)
	if metadataBundle != "" {
		resolvedContext.Metadata.Bundle = metadataBundle
		return nil
	}

	metadataPath = strings.TrimSpace(metadataPath)
	if metadataPath == "" {
		return nil
	}
	if strings.HasSuffix(metadataPath, ".tar.gz") || strings.HasSuffix(metadataPath, ".tgz") {
		resolvedContext.Metadata.Bundle = metadataPath
		return nil
	}
	info, err := os.Stat(metadataPath)
	if err != nil {
		return fmt.Errorf("resolve metadata artifact %q: %w", metadataPath, err)
	}
	if info.IsDir() {
		resolvedContext.Metadata.BaseDir = metadataPath
		return nil
	}
	return fmt.Errorf("metadata artifact %q must be a .tar.gz bundle or directory", metadataPath)
}

// resolveManagedServiceBundleRef fetches the referenced MetadataBundle CR and
// returns the source string consumed by the existing bundle resolver pipeline.
// Returns the empty string when no bundleRef is set, indicating the caller
// should fall back to the legacy metadata.url flow.
func resolveManagedServiceBundleRef(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	managedService *declarestv1alpha1.ManagedService,
) (string, error) {
	if managedService == nil || managedService.Spec.Metadata.BundleRef == nil {
		return "", nil
	}
	refName := strings.TrimSpace(managedService.Spec.Metadata.BundleRef.Name)
	if refName == "" {
		return "", nil
	}

	bundle := &declarestv1alpha1.MetadataBundle{}
	key := types.NamespacedName{Namespace: namespace, Name: refName}
	if err := reader.Get(ctx, key, bundle); err != nil {
		return "", fmt.Errorf("resolve metadata bundle %s/%s: %w", namespace, refName, err)
	}
	if !metadataBundleReady(bundle) {
		return "", fmt.Errorf("metadata bundle %s/%s is not ready", namespace, refName)
	}

	if cachePath := strings.TrimSpace(bundle.Status.CachePath); cachePath != "" {
		return cachePath, nil
	}
	// Fall back to the original source spec when the reconciler has not yet
	// persisted a cache path. The downstream resolver accepts shorthand and
	// URL forms directly.
	if shorthand := strings.TrimSpace(bundle.Spec.Source.Shorthand); shorthand != "" {
		return shorthand, nil
	}
	if url := strings.TrimSpace(bundle.Spec.Source.URL); url != "" {
		return url, nil
	}
	return "", fmt.Errorf("metadata bundle %s/%s has no resolvable source", namespace, refName)
}

func metadataBundleReady(bundle *declarestv1alpha1.MetadataBundle) bool {
	if bundle == nil {
		return false
	}
	for _, cond := range bundle.Status.Conditions {
		if cond.Type == declarestv1alpha1.ConditionTypeReady {
			return cond.Status == metav1.ConditionTrue
		}
	}
	return false
}
