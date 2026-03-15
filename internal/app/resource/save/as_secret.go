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

package save

import (
	"context"

	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	secretworkflow "github.com/crmarques/declarest/internal/app/secret/workflow"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

func saveResolvedPathAsSecret(
	ctx context.Context,
	deps Dependencies,
	writer orchestratordomain.RepositoryWriter,
	logicalPath string,
	content resource.Content,
) error {
	if _, err := appdeps.RequireMetadataService(deps); err != nil {
		return err
	}

	secretProvider, err := appdeps.RequireSecretProvider(deps)
	if err != nil {
		return err
	}

	secretValue, err := secretdomain.EncodeWholeResourceSecret(content)
	if err != nil {
		return err
	}

	secretKey, err := saveWholeResourceSecretKey(logicalPath)
	if err != nil {
		return err
	}
	if err := secretProvider.Store(ctx, secretKey, secretValue); err != nil {
		return err
	}

	if err := writer.Save(ctx, logicalPath, wholeResourceSecretPlaceholderContent(content.Descriptor)); err != nil {
		return err
	}

	return persistWholeResourceSecretMetadata(ctx, deps, logicalPath)
}

func saveWholeResourceSecretKey(logicalPath string) (string, error) {
	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return "", err
	}
	return secretworkflow.BuildPathScopedSecretKey(normalizedPath, "."), nil
}

func wholeResourceSecretPlaceholderContent(descriptor resource.PayloadDescriptor) resource.Content {
	resolvedDescriptor := resource.NormalizePayloadDescriptor(descriptor)
	placeholder := secretworkflow.PlaceholderValue()
	if resource.IsBinaryPayloadType(resolvedDescriptor.PayloadType) {
		return resource.Content{
			Value:      resource.BinaryValue{Bytes: []byte(placeholder)},
			Descriptor: resolvedDescriptor,
		}
	}
	return resource.Content{
		Value:      placeholder,
		Descriptor: resolvedDescriptor,
	}
}

func persistWholeResourceSecretMetadata(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
) error {
	metadataService, err := appdeps.RequireMetadataService(deps)
	if err != nil {
		return err
	}

	currentMetadata, err := metadataService.Get(ctx, logicalPath)
	if err != nil {
		if !faults.IsCategory(err, faults.NotFoundError) {
			return err
		}
		currentMetadata = metadatadomain.ResourceMetadata{}
	}

	wholeSecret := true
	currentMetadata.Secret = &wholeSecret
	currentMetadata.SecretAttributes = nil

	return metadataService.Set(ctx, logicalPath, currentMetadata)
}
