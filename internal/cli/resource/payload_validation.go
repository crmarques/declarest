package resource

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	metadatadomain "github.com/crmarques/declarest/metadata"
	resourcedomain "github.com/crmarques/declarest/resource"
	identitysupport "github.com/crmarques/declarest/resource/identity"
)

func resolveExplicitMutationPayloadPath(
	ctx context.Context,
	commandPath string,
	deps cliutil.CommandDependencies,
	logicalPath string,
	content resourcedomain.Content,
) (string, error) {
	normalizedPath, err := resourcedomain.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return "", err
	}
	if deps.Services == nil || deps.Services.MetadataService() == nil {
		return normalizedPath, nil
	}
	payloadMap, ok := content.Value.(map[string]any)
	if !ok {
		return normalizedPath, nil
	}

	md, err := deps.Services.MetadataService().ResolveForPath(ctx, normalizedPath)
	if err != nil {
		return "", err
	}

	validationErr := validateExplicitMutationPayloadIdentityForPath(commandPath, normalizedPath, payloadMap, md)
	if validationErr == nil {
		return normalizedPath, nil
	}
	canInfer, err := canInferExplicitMutationChildPath(ctx, deps, normalizedPath)
	if err != nil {
		return "", err
	}
	if !canInfer {
		return "", validationErr
	}

	identitySegment, ok := explicitMutationPayloadIdentitySegment(payloadMap, md)
	if !ok {
		// Keep the original validation error when payload does not expose a
		// usable identity for collection-target inference.
		return "", validationErr
	}

	inferredPath, err := resourcedomain.JoinLogicalPath(normalizedPath, identitySegment)
	if err != nil {
		return "", err
	}

	if err := validateExplicitMutationPayloadIdentityForPath(commandPath, inferredPath, payloadMap, md); err != nil {
		return "", err
	}
	return inferredPath, nil
}

func validateExplicitMutationPayloadIdentity(
	ctx context.Context,
	commandPath string,
	deps cliutil.CommandDependencies,
	logicalPath string,
	content resourcedomain.Content,
) error {
	normalizedPath, err := resourcedomain.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}
	if deps.Services == nil || deps.Services.MetadataService() == nil {
		return nil
	}

	payloadMap, ok := content.Value.(map[string]any)
	if !ok {
		return nil
	}

	md, err := deps.Services.MetadataService().ResolveForPath(ctx, normalizedPath)
	if err != nil {
		return err
	}
	return validateExplicitMutationPayloadIdentityForPath(commandPath, normalizedPath, payloadMap, md)
}

func validateExplicitMutationPayloadIdentityForPath(
	commandPath string,
	normalizedPath string,
	payloadMap map[string]any,
	md metadatadomain.ResourceMetadata,
) error {
	pathSegment := strings.TrimSpace(path.Base(strings.TrimSuffix(normalizedPath, "/")))
	if pathSegment == "" || pathSegment == "/" {
		return nil
	}

	if strings.TrimSpace(md.Alias) == "" && strings.TrimSpace(md.ID) == "" {
		return nil
	}

	alias, remoteID, err := identitysupport.ResolveAliasAndRemoteIDForListItem(payloadMap, md)
	if err != nil {
		return nil
	}

	identityKind := "resource.id"
	identityValue := strings.TrimSpace(remoteID)
	if strings.TrimSpace(md.Alias) != "" {
		identityKind = "resource.alias"
		identityValue = strings.TrimSpace(alias)
	}
	if identityValue == "" || identityValue == pathSegment {
		return nil
	}

	return cliutil.ValidationError(
		fmt.Sprintf(
			"%s explicit payload %s value %q does not match path segment %q for %q",
			strings.TrimSpace(commandPath),
			identityKind,
			identityValue,
			pathSegment,
			normalizedPath,
		),
		nil,
	)
}

func canInferExplicitMutationChildPath(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	normalizedPath string,
) (bool, error) {
	if deps.Services == nil || deps.Services.MetadataService() == nil {
		return false, nil
	}

	wildcardResolver, ok := deps.Services.MetadataService().(metadatadomain.CollectionWildcardResolver)
	if !ok {
		return false, nil
	}

	hasWildcard, err := wildcardResolver.HasCollectionWildcardChild(ctx, normalizedPath)
	if err != nil {
		return false, err
	}
	return hasWildcard, nil
}

func explicitMutationPayloadIdentitySegment(
	payload map[string]any,
	md metadatadomain.ResourceMetadata,
) (string, bool) {
	if strings.TrimSpace(md.Alias) == "" && strings.TrimSpace(md.ID) == "" {
		return "", false
	}
	alias, remoteID, err := identitysupport.ResolveAliasAndRemoteIDForListItem(payload, md)
	if err != nil {
		return "", false
	}
	if strings.TrimSpace(alias) != "" {
		return alias, true
	}
	if strings.TrimSpace(remoteID) != "" {
		return remoteID, true
	}
	return "", false
}
