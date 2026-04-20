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

package bootstrap

import (
	"context"

	bundlemetadata "github.com/crmarques/declarest/internal/providers/metadata/bundle"
)

// ResolvedMetadataBundle is the boundary-safe view of a resolved bundle that
// controllers and other non-provider callers are allowed to consume. Domain
// code depends only on this type, not on the provider implementation.
type ResolvedMetadataBundle struct {
	MetadataDir         string
	OpenAPI             string
	Name                string
	Version             string
	Description         string
	MetadataRoot        string
	DeclarestOpenAPI    string
	CompatibleDeclarest string
	CompatibleProduct   string
	CompatibleVersions  string
	Deprecated          bool
	DeprecatedWarning   string
}

// ResolveMetadataBundle wraps the provider-level `bundlemetadata.ResolveBundle`
// behind a bootstrap-owned surface so the boundary rule (controllers MUST NOT
// import `internal/providers/...`) is preserved.
func ResolveMetadataBundle(ctx context.Context, ref string) (ResolvedMetadataBundle, error) {
	resolution, err := bundlemetadata.ResolveBundle(ctx, ref)
	if err != nil {
		return ResolvedMetadataBundle{}, err
	}
	return ResolvedMetadataBundle{
		MetadataDir:         resolution.MetadataDir,
		OpenAPI:             resolution.OpenAPI,
		Name:                resolution.Manifest.Name,
		Version:             resolution.Manifest.Version,
		Description:         resolution.Manifest.Description,
		MetadataRoot:        resolution.Manifest.Declarest.MetadataRoot,
		DeclarestOpenAPI:    resolution.Manifest.Declarest.OpenAPI,
		CompatibleDeclarest: resolution.Manifest.Declarest.CompatibleDeclarest,
		CompatibleProduct:   resolution.Manifest.Declarest.CompatibleManagedService.Product,
		CompatibleVersions:  resolution.Manifest.Declarest.CompatibleManagedService.Versions,
		Deprecated:          resolution.Manifest.Deprecated,
		DeprecatedWarning:   resolution.DeprecatedWarning,
	}, nil
}
