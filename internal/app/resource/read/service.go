package read

import (
	"context"
	"errors"
	"fmt"
	"strings"

	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	appdeps "github.com/crmarques/declarest/internal/app/deps"
	"github.com/crmarques/declarest/internal/app/resource/pathfallback"
	secretworkflow "github.com/crmarques/declarest/internal/app/secret/workflow"
	managedserverdomain "github.com/crmarques/declarest/managedserver"
	metadatadomain "github.com/crmarques/declarest/metadata"
	metadataRender "github.com/crmarques/declarest/metadata/render"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

const (
	SourceRepository    = "repository"
	SourceManagedServer = "managed-server"
)

type Dependencies = appdeps.Dependencies

type Request struct {
	LogicalPath              string
	Source                   string
	SkipItems                []string
	ExplicitCollectionTarget bool
	ShowSecrets              bool
	ShowMetadata             bool
	ContextName              string
}

type Result struct {
	OutputValue any
	TextLines   []string
}

type OutputWithMetadata struct {
	Payload  resource.Value                  `json:"payload" yaml:"payload"`
	Metadata metadatadomain.ResourceMetadata `json:"metadata" yaml:"metadata"`
}

func Execute(ctx context.Context, deps Dependencies, req Request) (Result, error) {
	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return Result{}, err
	}

	debugctx.Printf(ctx, "resource read requested path=%q source=%q", req.LogicalPath, req.Source)

	if req.Source == SourceManagedServer && req.ExplicitCollectionTarget {
		debugctx.Printf(ctx, "resource read treating %q as remote collection listing", req.LogicalPath)
		result, err := renderRemoteCollection(ctx, deps, orchestratorService, req.LogicalPath, req.ShowSecrets, req.SkipItems)
		if err == nil {
			return result, nil
		}
		if !managedserverdomain.IsListPayloadShapeError(err) {
			return Result{}, err
		}
		debugctx.Printf(
			ctx,
			"resource read falling back to single-resource remote read for %q after collection-list shape error: %v",
			req.LogicalPath,
			err,
		)
	}

	var content resource.Content
	switch req.Source {
	case SourceRepository:
		content, err = orchestratorService.GetLocal(ctx, req.LogicalPath)
	case SourceManagedServer:
		content, err = orchestratorService.GetRemote(ctx, req.LogicalPath)
	default:
		return Result{}, faults.NewValidationError("invalid source: use --source repository|managed-server", nil)
	}
	if err != nil {
		debugctx.Printf(ctx, "resource read failed path=%q source=%q error=%v", req.LogicalPath, req.Source, err)

		if req.Source == SourceRepository && (isNotFoundError(err) || isRootResourceError(err)) {
			debugctx.Printf(ctx, "resource read treating %q as repository collection listing", req.LogicalPath)
			return renderRepositoryCollection(ctx, deps, orchestratorService, req.LogicalPath, req.ShowSecrets, req.SkipItems)
		}
		if req.Source == SourceManagedServer && !req.ExplicitCollectionTarget && isNotFoundError(err) {
			debugctx.Printf(ctx, "resource read attempting empty-collection fallback for %q after remote not found", req.LogicalPath)
			handled, fallbackResult, fallbackErr := renderRemoteCollectionFallback(
				ctx,
				deps,
				orchestratorService,
				req.LogicalPath,
				req.ShowSecrets,
				req.SkipItems,
			)
			if fallbackErr == nil && handled {
				return fallbackResult, nil
			}
			if fallbackErr != nil {
				debugctx.Printf(ctx, "resource read empty-collection fallback failed for %q error=%v", req.LogicalPath, fallbackErr)
			}
		}
		return Result{}, err
	}

	debugctx.Printf(ctx, "resource read succeeded path=%q value_type=%T source=%q", req.LogicalPath, content.Value, req.Source)

	rawValue := content.Value
	var metadataSnapshot *metadatadomain.ResourceMetadata
	if req.ShowMetadata {
		snapshot, err := renderMetadataSnapshot(ctx, deps, req.LogicalPath, rawValue)
		if err != nil {
			return Result{}, err
		}
		metadataSnapshot = &snapshot
	}

	finalValue, err := prepareSecretsForOutput(ctx, deps, req.LogicalPath, rawValue, content.Descriptor, req.ShowSecrets)
	if err != nil {
		return Result{}, err
	}

	outputValue := any(finalValue)
	if req.ShowMetadata && metadataSnapshot != nil {
		outputValue = OutputWithMetadata{
			Payload:  finalValue,
			Metadata: *metadataSnapshot,
		}
	}

	return Result{OutputValue: outputValue}, nil
}

func renderRepositoryCollection(
	ctx context.Context,
	deps Dependencies,
	orchestratorService orchestrator.LocalReader,
	logicalPath string,
	showSecrets bool,
	skipItems []string,
) (Result, error) {
	items, err := orchestratorService.ListLocal(ctx, logicalPath, orchestrator.ListPolicy{})
	if err != nil {
		return Result{}, err
	}
	return renderCollection(ctx, deps, logicalPath, items, showSecrets, skipItems)
}

func renderRemoteCollection(
	ctx context.Context,
	deps Dependencies,
	orchestratorService orchestrator.RemoteReader,
	logicalPath string,
	showSecrets bool,
	skipItems []string,
) (Result, error) {
	items, err := orchestratorService.ListRemote(ctx, logicalPath, orchestrator.ListPolicy{})
	if err != nil {
		return Result{}, err
	}
	return renderCollection(ctx, deps, logicalPath, items, showSecrets, skipItems)
}

func renderRemoteCollectionFallback(
	ctx context.Context,
	deps Dependencies,
	orchestratorService orchestrator.RemoteReader,
	logicalPath string,
	showSecrets bool,
	skipItems []string,
) (bool, Result, error) {
	items, err := orchestratorService.ListRemote(ctx, logicalPath, orchestrator.ListPolicy{})
	if err != nil {
		return false, Result{}, err
	}
	if !pathfallback.ShouldUseMetadataCollectionFallback(ctx, deps.Metadata, logicalPath, items) {
		return false, Result{}, nil
	}

	result, err := renderCollection(ctx, deps, logicalPath, items, showSecrets, skipItems)
	if err != nil {
		return true, Result{}, err
	}
	return true, result, nil
}

func renderCollection(
	ctx context.Context,
	deps Dependencies,
	collectionPath string,
	items []resource.Resource,
	showSecrets bool,
	skipItems []string,
) (Result, error) {
	items = resource.FilterCollectionItems(collectionPath, items, skipItems)
	if !showSecrets {
		maskedItems := make([]resource.Resource, 0, len(items))
		for _, item := range items {
			maskedPayload, err := maskSecretsForOutput(ctx, deps, item.LogicalPath, item.Payload)
			if err != nil {
				return Result{}, err
			}
			item.Payload = maskedPayload
			maskedItems = append(maskedItems, item)
		}
		items = maskedItems
	} else {
		resolvedItems := make([]resource.Resource, 0, len(items))
		for _, item := range items {
			resolvedPayload, err := resolveSecretsForOutput(ctx, deps, item.LogicalPath, item.Payload, item.PayloadDescriptor)
			if err != nil {
				return Result{}, err
			}
			item.Payload = resolvedPayload
			resolvedItems = append(resolvedItems, item)
		}
		items = resolvedItems
	}

	payloads := make([]resource.Value, len(items))
	lines := make([]string, 0, len(items))
	for i, item := range items {
		payloads[i] = item.Payload
		lines = append(lines, item.LogicalPath)
	}

	return Result{
		OutputValue: payloads,
		TextLines:   lines,
	}, nil
}

func prepareSecretsForOutput(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	value resource.Value,
	descriptor resource.PayloadDescriptor,
	showSecrets bool,
) (resource.Value, error) {
	if showSecrets {
		return resolveSecretsForOutput(ctx, deps, logicalPath, value, descriptor)
	}
	return maskSecretsForOutput(ctx, deps, logicalPath, value)
}

func maskSecretsForOutput(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	value resource.Value,
) (resource.Value, error) {
	if value == nil {
		return nil, nil
	}

	resolvedMetadata, err := secretworkflow.ResolveMetadataForSecretCheck(ctx, deps.Metadata, logicalPath)
	if err != nil {
		return nil, err
	}
	if resolvedMetadata.IsWholeResourceSecret() {
		if isWholeResourceSecretPlaceholderValue(value) {
			return value, nil
		}
		return secretworkflow.PlaceholderValue(), nil
	}

	secretAttributes := secretworkflow.DedupeAndSortAttributes(resolvedMetadata.SecretsFromAttributes)
	if len(secretAttributes) == 0 {
		return value, nil
	}
	return secretworkflow.MaskValue(value, secretAttributes)
}

func isWholeResourceSecretPlaceholderValue(value resource.Value) bool {
	switch typed := value.(type) {
	case string:
		return secretworkflow.IsPlaceholderValue(typed)
	case resource.BinaryValue:
		return secretworkflow.IsPlaceholderValue(string(typed.Bytes))
	case *resource.BinaryValue:
		return typed != nil && secretworkflow.IsPlaceholderValue(string(typed.Bytes))
	default:
		return false
	}
}

func resolveSecretsForOutput(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	value resource.Value,
	descriptor resource.PayloadDescriptor,
) (resource.Value, error) {
	if value == nil {
		return nil, nil
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}

	getSecret := func(string) (string, error) {
		return "", faults.NewValidationError(
			"flag --show-secrets requires a configured secret provider when payload includes placeholders",
			nil,
		)
	}
	if deps.Secrets != nil {
		getSecret = func(key string) (string, error) {
			return deps.Secrets.Get(ctx, key)
		}
	}

	if resolvedWholeResource, handled, err := secretdomain.ResolveWholeResourcePlaceholderForResource(
		value,
		normalizedPath,
		descriptor,
		getSecret,
	); handled || err != nil {
		return resolvedWholeResource, err
	}

	return secretdomain.ResolvePayloadForResource(value, normalizedPath, getSecret)
}

func renderMetadataSnapshot(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	rawValue resource.Value,
) (metadatadomain.ResourceMetadata, error) {
	if deps.Metadata == nil {
		return metadatadomain.ResourceMetadata{}, faults.NewValidationError("metadata service is not configured", nil)
	}

	resolvedMetadata, err := deps.Metadata.ResolveForPath(ctx, logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	merged := metadatadomain.MergeResourceMetadata(
		metadatadomain.DefaultResourceMetadata(),
		resolvedMetadata,
	)

	return metadataRender.RenderResourceMetadata(ctx, logicalPath, merged, rawValue)
}

func isNotFoundError(err error) bool {
	var typedErr *faults.TypedError
	return errors.As(err, &typedErr) && typedErr.Category == faults.NotFoundError
}

func isRootResourceError(err error) bool {
	var typedErr *faults.TypedError
	return errors.As(err, &typedErr) &&
		typedErr.Category == faults.ValidationError &&
		strings.TrimSpace(typedErr.Message) == "logical path must target a resource, not root"
}

func HasCollectionTargetMarker(rawPath string) bool {
	return resource.HasExplicitCollectionTarget(rawPath)
}

func RenderTextLines(lines []string) func(any) []string {
	return func(any) []string { return lines }
}

func (r Result) HasTextLines() bool {
	return r.TextLines != nil
}

func (r Result) String() string {
	return fmt.Sprintf("read.Result{output:%T,lines:%d}", r.OutputValue, len(r.TextLines))
}
