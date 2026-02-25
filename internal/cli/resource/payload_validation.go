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

func validateExplicitMutationPayloadIdentity(
	ctx context.Context,
	commandPath string,
	deps common.CommandDependencies,
	logicalPath string,
	value resourcedomain.Value,
) error {
	if deps.Metadata == nil {
		return nil
	}

	normalizedPath, err := resourcedomain.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return err
	}
	payloadMap, ok := value.(map[string]any)
	if !ok {
		return nil
	}

	md, err := deps.Metadata.ResolveForPath(ctx, normalizedPath)
	if err != nil {
		return err
	}

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
