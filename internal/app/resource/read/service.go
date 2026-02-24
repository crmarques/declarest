package read

import (
	"context"
	"errors"
	"fmt"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	debugctx "github.com/crmarques/declarest/debugctx"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/app/resource/pathfallback"
	secretworkflow "github.com/crmarques/declarest/internal/app/secret/workflow"
	metadatadomain "github.com/crmarques/declarest/metadata"
	metadataRender "github.com/crmarques/declarest/metadata/render"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
	serverdomain "github.com/crmarques/declarest/server"
)

const (
	SourceRepository   = "repository"
	SourceRemoteServer = "remote-server"
)

type Dependencies struct {
	Orchestrator orchestrator.Orchestrator
	Contexts     configdomain.ContextService
	Metadata     metadatadomain.MetadataService
	Secrets      secretdomain.SecretProvider
}

type Request struct {
	LogicalPath              string
	Source                   string
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
	orchestratorService, err := requireOrchestrator(deps)
	if err != nil {
		return Result{}, err
	}

	debugctx.Printf(ctx, "resource read requested path=%q source=%q", req.LogicalPath, req.Source)

	if req.Source == SourceRemoteServer && req.ExplicitCollectionTarget {
		debugctx.Printf(ctx, "resource read treating %q as remote collection listing", req.LogicalPath)
		result, err := renderRemoteCollection(ctx, deps, orchestratorService, req.LogicalPath, req.ShowSecrets)
		if err == nil {
			return result, nil
		}
		if !serverdomain.IsListPayloadShapeError(err) {
			return Result{}, err
		}
		debugctx.Printf(
			ctx,
			"resource read falling back to single-resource remote read for %q after collection-list shape error: %v",
			req.LogicalPath,
			err,
		)
	}

	var value resource.Value
	switch req.Source {
	case SourceRepository:
		value, err = orchestratorService.GetLocal(ctx, req.LogicalPath)
	case SourceRemoteServer:
		value, err = orchestratorService.GetRemote(ctx, req.LogicalPath)
	default:
		return Result{}, validationError("invalid source: use --repository or --remote-server", nil)
	}
	if err != nil {
		debugctx.Printf(ctx, "resource read failed path=%q source=%q error=%v", req.LogicalPath, req.Source, err)

		if req.Source == SourceRepository && (isNotFoundError(err) || isRootResourceError(err)) {
			debugctx.Printf(ctx, "resource read treating %q as repository collection listing", req.LogicalPath)
			return renderRepositoryCollection(ctx, deps, orchestratorService, req.LogicalPath, req.ShowSecrets)
		}
		if req.Source == SourceRemoteServer && !req.ExplicitCollectionTarget && isNotFoundError(err) {
			debugctx.Printf(ctx, "resource read attempting empty-collection fallback for %q after remote not found", req.LogicalPath)
			handled, fallbackResult, fallbackErr := renderRemoteCollectionFallback(ctx, deps, orchestratorService, req.LogicalPath, req.ShowSecrets)
			if fallbackErr == nil && handled {
				return fallbackResult, nil
			}
			if fallbackErr != nil {
				debugctx.Printf(ctx, "resource read empty-collection fallback failed for %q error=%v", req.LogicalPath, fallbackErr)
			}
		}
		return Result{}, err
	}

	debugctx.Printf(ctx, "resource read succeeded path=%q value_type=%T source=%q", req.LogicalPath, value, req.Source)

	rawValue := value
	var metadataSnapshot *metadatadomain.ResourceMetadata
	if req.ShowMetadata {
		snapshot, err := renderMetadataSnapshot(ctx, deps, req.LogicalPath, rawValue, req.ContextName)
		if err != nil {
			return Result{}, err
		}
		metadataSnapshot = &snapshot
	}

	finalValue, err := prepareSecretsForOutput(ctx, deps, req.LogicalPath, rawValue, req.ShowSecrets)
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
) (Result, error) {
	items, err := orchestratorService.ListLocal(ctx, logicalPath, orchestrator.ListPolicy{})
	if err != nil {
		return Result{}, err
	}
	return renderCollection(ctx, deps, items, showSecrets)
}

func renderRemoteCollection(
	ctx context.Context,
	deps Dependencies,
	orchestratorService orchestrator.RemoteReader,
	logicalPath string,
	showSecrets bool,
) (Result, error) {
	items, err := orchestratorService.ListRemote(ctx, logicalPath, orchestrator.ListPolicy{})
	if err != nil {
		return Result{}, err
	}
	return renderCollection(ctx, deps, items, showSecrets)
}

func renderRemoteCollectionFallback(
	ctx context.Context,
	deps Dependencies,
	orchestratorService orchestrator.RemoteReader,
	logicalPath string,
	showSecrets bool,
) (bool, Result, error) {
	items, err := orchestratorService.ListRemote(ctx, logicalPath, orchestrator.ListPolicy{})
	if err != nil {
		return false, Result{}, err
	}
	if !pathfallback.ShouldUseMetadataCollectionFallback(ctx, deps.Metadata, logicalPath, items) {
		return false, Result{}, nil
	}

	result, err := renderCollection(ctx, deps, items, showSecrets)
	if err != nil {
		return true, Result{}, err
	}
	return true, result, nil
}

func renderCollection(ctx context.Context, deps Dependencies, items []resource.Resource, showSecrets bool) (Result, error) {
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
			resolvedPayload, err := resolveSecretsForOutput(ctx, deps, item.LogicalPath, item.Payload)
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
	showSecrets bool,
) (resource.Value, error) {
	if showSecrets {
		return resolveSecretsForOutput(ctx, deps, logicalPath, value)
	}
	return maskSecretsForOutput(ctx, deps, logicalPath, value)
}

func maskSecretsForOutput(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	value resource.Value,
) (resource.Value, error) {
	secretAttributes, err := secretworkflow.ResolveDeclaredAttributes(ctx, deps.Metadata, logicalPath)
	if err != nil {
		return nil, err
	}
	if len(secretAttributes) == 0 {
		return value, nil
	}
	return secretworkflow.MaskValue(value, secretAttributes)
}

func resolveSecretsForOutput(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	value resource.Value,
) (resource.Value, error) {
	if value == nil {
		return nil, nil
	}

	normalizedPath, err := resource.NormalizeLogicalPath(logicalPath)
	if err != nil {
		return nil, err
	}

	if deps.Secrets == nil {
		return secretdomain.ResolvePayloadForResource(value, normalizedPath, func(string) (string, error) {
			return "", validationError(
				"flag --show-secrets requires a configured secret provider when payload includes placeholders",
				nil,
			)
		})
	}

	return secretdomain.ResolvePayloadForResource(value, normalizedPath, func(key string) (string, error) {
		return deps.Secrets.Get(ctx, key)
	})
}

func renderMetadataSnapshot(
	ctx context.Context,
	deps Dependencies,
	logicalPath string,
	rawValue resource.Value,
	contextName string,
) (metadatadomain.ResourceMetadata, error) {
	if deps.Metadata == nil {
		return metadatadomain.ResourceMetadata{}, validationError("metadata service is not configured", nil)
	}

	resolvedMetadata, err := deps.Metadata.ResolveForPath(ctx, logicalPath)
	if err != nil {
		return metadatadomain.ResourceMetadata{}, err
	}
	merged := metadatadomain.MergeResourceMetadata(
		metadatadomain.DefaultResourceMetadata(),
		resolvedMetadata,
	)

	resourceFormat := configdomain.ResourceFormatJSON
	if deps.Contexts != nil {
		resolvedContext, err := deps.Contexts.ResolveContext(ctx, configdomain.ContextSelection{Name: contextName})
		if err != nil {
			return metadatadomain.ResourceMetadata{}, err
		}
		resourceFormat = resolvedContext.Repository.ResourceFormat
	}

	return metadataRender.RenderResourceMetadataWithFormat(ctx, logicalPath, merged, rawValue, resourceFormat)
}

func requireOrchestrator(deps Dependencies) (orchestrator.Orchestrator, error) {
	if deps.Orchestrator == nil {
		return nil, validationError("orchestrator is not configured", nil)
	}
	return deps.Orchestrator, nil
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
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
	trimmed := strings.TrimSpace(rawPath)
	return trimmed != "" && trimmed != "/" && strings.HasSuffix(trimmed, "/")
}

func NormalizeSource(fromRepository bool, fromRemoteServer bool) (string, error) {
	if fromRepository && fromRemoteServer {
		return "", validationError("flags --repository and --remote-server cannot be used together", nil)
	}
	if fromRepository {
		return SourceRepository, nil
	}
	return SourceRemoteServer, nil
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
