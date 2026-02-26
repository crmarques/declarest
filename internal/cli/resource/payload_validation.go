package resource

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/crmarques/declarest/internal/cli/common"
	metadatadomain "github.com/crmarques/declarest/metadata"
	resourcedomain "github.com/crmarques/declarest/resource"
	identitysupport "github.com/crmarques/declarest/resource/identity"
)

func resolveExplicitMutationPayloadPath(
	ctx context.Context,
	commandPath string,
	deps common.CommandDependencies,
	logicalPath string,
	value resourcedomain.Value,
) (string, error) {
	normalizedPath, err := resourcedomain.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return "", err
	}
	if deps.Metadata == nil {
		return normalizedPath, nil
	}
	payloadMap, ok := value.(map[string]any)
	if !ok {
		return normalizedPath, nil
	}

	md, err := deps.Metadata.ResolveForPath(ctx, normalizedPath)
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
	deps common.CommandDependencies,
	logicalPath string,
	value resourcedomain.Value,
) error {
	normalizedPath, err := resourcedomain.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}
	if deps.Metadata == nil {
		return nil
	}

	payloadMap, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	md, err := deps.Metadata.ResolveForPath(ctx, normalizedPath)
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

	if err := validatePayloadIdentityAttributeMatch(commandPath, normalizedPath, pathSegment, payloadMap, md, true); err != nil {
		return err
	}
	if err := validatePayloadIdentityAttributeMatch(commandPath, normalizedPath, pathSegment, payloadMap, md, false); err != nil {
		return err
	}

	return nil
}

func canInferExplicitMutationChildPath(
	ctx context.Context,
	deps common.CommandDependencies,
	normalizedPath string,
) (bool, error) {
	if deps.Metadata == nil {
		return false, nil
	}

	wildcardResolver, ok := deps.Metadata.(metadatadomain.CollectionWildcardResolver)
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
	candidates := []string{
		strings.TrimSpace(md.AliasFromAttribute),
		strings.TrimSpace(md.IDFromAttribute),
	}

	for _, attributeName := range candidates {
		if attributeName == "" {
			continue
		}
		value, found := identitysupport.LookupScalarAttribute(payload, attributeName)
		value = strings.TrimSpace(value)
		if !found || value == "" {
			continue
		}
		if strings.Contains(value, "/") {
			return "", false
		}
		return value, true
	}

	return "", false
}

func validatePayloadIdentityAttributeMatch(
	commandPath string,
	normalizedPath string,
	pathSegment string,
	payload map[string]any,
	md metadatadomain.ResourceMetadata,
	checkAlias bool,
) error {
	attributeName := strings.TrimSpace(md.IDFromAttribute)
	identityKind := "id"
	if checkAlias {
		attributeName = strings.TrimSpace(md.AliasFromAttribute)
		identityKind = "alias"
	}
	if attributeName == "" {
		return nil
	}

	// When alias and id attributes are distinct, the logical path segment is
	// expected to follow alias semantics, not remote-id semantics.
	if !checkAlias && strings.TrimSpace(md.AliasFromAttribute) != "" && strings.TrimSpace(md.AliasFromAttribute) != attributeName {
		return nil
	}

	payloadValue, found := identitysupport.LookupScalarAttribute(payload, attributeName)
	if !found || strings.TrimSpace(payloadValue) == "" {
		return nil
	}

	if strings.TrimSpace(payloadValue) == pathSegment {
		return nil
	}

	return common.ValidationError(
		fmt.Sprintf(
			"%s explicit payload %s attribute %q=%q does not match path segment %q for %q",
			strings.TrimSpace(commandPath),
			identityKind,
			attributeName,
			strings.TrimSpace(payloadValue),
			pathSegment,
			normalizedPath,
		),
		nil,
	)
}
