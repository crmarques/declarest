package save

import (
	"context"

	appdeps "github.com/crmarques/declarest/internal/app/deps"
	secretworkflow "github.com/crmarques/declarest/internal/app/secret/workflow"
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

	return writer.Save(ctx, logicalPath, wholeResourceSecretPlaceholderContent(content.Descriptor))
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
