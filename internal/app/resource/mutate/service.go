package mutate

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	resourcesave "github.com/crmarques/declarest/internal/app/resource/save"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
)

type Operation string

const (
	OperationApply  Operation = "apply"
	OperationCreate Operation = "create"
	OperationUpdate Operation = "update"
)

type Dependencies struct {
	Orchestrator orchestratordomain.Orchestrator
	Repository   repository.ResourceStore
	Metadata     metadatadomain.MetadataService
	Secrets      secretdomain.SecretProvider
}

type Request struct {
	Operation        Operation
	LogicalPath      string
	Recursive        bool
	Force            bool
	Value            resource.Content
	HasExplicitInput bool
	RefreshLocal     bool
}

type Result struct {
	ResolvedPath  string
	TargetedCount int
	Items         []resource.Resource
}

func Execute(ctx context.Context, deps Dependencies, req Request) (Result, error) {
	orchestratorService, err := requireOrchestrator(deps)
	if err != nil {
		return Result{}, err
	}

	if req.HasExplicitInput {
		if req.Recursive {
			return Result{}, faults.NewValidationError(
				fmt.Sprintf(
					"flag --recursive cannot be combined with explicit input; remove input to %s resources from repository",
					strings.TrimSpace(string(req.Operation)),
				),
				nil,
			)
		}

		item, err := runExplicitMutation(ctx, orchestratorService, req.Operation, req.LogicalPath, req.Value, req.Force)
		if err != nil {
			return Result{}, err
		}
		items := []resource.Resource{item}
		if req.RefreshLocal {
			if err := refreshRepositoryForPaths(ctx, deps, items); err != nil {
				return Result{}, err
			}
		}

		return Result{ResolvedPath: req.LogicalPath, TargetedCount: 1, Items: items}, nil
	}

	targets, err := ListLocalTargets(ctx, orchestratorService, req.LogicalPath, req.Recursive)
	if err != nil {
		return Result{}, err
	}
	targetedCount := len(targets)

	items, err := executeMutationForTargets(ctx, targets, func(runCtx context.Context, logicalPath string) (resource.Resource, error) {
		switch req.Operation {
		case OperationApply:
			return orchestratorService.Apply(runCtx, logicalPath, orchestratordomain.ApplyPolicy{
				Force: req.Force,
			})
		case OperationCreate:
			localValue, getErr := orchestratorService.GetLocal(runCtx, logicalPath)
			if getErr != nil {
				return resource.Resource{}, getErr
			}
			return orchestratorService.Create(runCtx, logicalPath, localValue)
		case OperationUpdate:
			localValue, getErr := orchestratorService.GetLocal(runCtx, logicalPath)
			if getErr != nil {
				return resource.Resource{}, getErr
			}
			return orchestratorService.Update(runCtx, logicalPath, localValue)
		default:
			return resource.Resource{}, faults.NewValidationError(
				fmt.Sprintf("unsupported resource mutation operation %q", req.Operation),
				nil,
			)
		}
	})
	if err != nil {
		return Result{}, err
	}

	if req.RefreshLocal {
		if err := refreshRepositoryForPaths(ctx, deps, items); err != nil {
			return Result{}, err
		}
	}

	return Result{ResolvedPath: req.LogicalPath, TargetedCount: targetedCount, Items: items}, nil
}

func runExplicitMutation(
	ctx context.Context,
	orchestratorService orchestratordomain.Orchestrator,
	operation Operation,
	logicalPath string,
	value resource.Content,
	force bool,
) (resource.Resource, error) {
	switch operation {
	case OperationApply:
		return orchestratorService.ApplyWithContent(ctx, logicalPath, value, orchestratordomain.ApplyPolicy{
			Force: force,
		})
	case OperationCreate:
		return orchestratorService.Create(ctx, logicalPath, value)
	case OperationUpdate:
		return orchestratorService.Update(ctx, logicalPath, value)
	default:
		return resource.Resource{}, faults.NewValidationError(
			fmt.Sprintf("unsupported resource mutation operation %q", operation),
			nil,
		)
	}
}

func ListLocalTargets(
	ctx context.Context,
	orchestratorService orchestratordomain.LocalReader,
	logicalPath string,
	recursive bool,
) ([]resource.Resource, error) {
	items, err := orchestratorService.ListLocal(ctx, logicalPath, orchestratordomain.ListPolicy{Recursive: recursive})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 && !recursive && logicalPathDepth(logicalPath) > 1 {
		localValue, getErr := orchestratorService.GetLocal(ctx, logicalPath)
		if getErr == nil {
			items = []resource.Resource{{
				LogicalPath:       logicalPath,
				Payload:           localValue.Value,
				PayloadDescriptor: localValue.Descriptor,
			}}
		} else if !faults.IsCategory(getErr, faults.NotFoundError) {
			return nil, getErr
		}
	}
	if len(items) == 0 {
		return nil, faults.NewTypedError(
			faults.NotFoundError,
			fmt.Sprintf("no local resources found under %q", logicalPath),
			nil,
		)
	}

	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})
	return items, nil
}

func ListLocalTargetsOrFallbackPath(
	ctx context.Context,
	orchestratorService orchestratordomain.LocalReader,
	logicalPath string,
	recursive bool,
) ([]resource.Resource, error) {
	items, err := ListLocalTargets(ctx, orchestratorService, logicalPath, recursive)
	if err == nil {
		return items, nil
	}
	if isRepositoryNotConfiguredValidation(err) {
		if recursive {
			return nil, faults.NewValidationError(
				"flag --recursive requires a configured repository to resolve delete targets",
				nil,
			)
		}
		return []resource.Resource{{LogicalPath: logicalPath}}, nil
	}
	if faults.IsCategory(err, faults.NotFoundError) {
		return []resource.Resource{{LogicalPath: logicalPath}}, nil
	}
	return nil, err
}

func executeMutationForTargets(
	ctx context.Context,
	targets []resource.Resource,
	runMutation func(context.Context, string) (resource.Resource, error),
) ([]resource.Resource, error) {
	results := make([]resource.Resource, 0, len(targets))
	for _, target := range targets {
		item, err := runMutation(ctx, target.LogicalPath)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	sort.Slice(results, func(i int, j int) bool {
		return results[i].LogicalPath < results[j].LogicalPath
	})
	return results, nil
}

func refreshRepositoryForPaths(ctx context.Context, deps Dependencies, items []resource.Resource) error {
	if len(items) == 0 {
		return nil
	}

	saveDeps := resourcesave.Dependencies{
		Orchestrator: deps.Orchestrator,
		Repository:   deps.Repository,
		Metadata:     deps.Metadata,
		Secrets:      deps.Secrets,
	}

	for _, item := range items {
		if err := resourcesave.Execute(
			ctx,
			saveDeps,
			item.LogicalPath,
			resource.Content{},
			false,
			resourcesave.ExecuteOptions{Force: true},
		); err != nil {
			return err
		}
	}
	return nil
}

func isRepositoryNotConfiguredValidation(err error) bool {
	if !faults.IsCategory(err, faults.ValidationError) {
		return false
	}

	message := strings.TrimSpace(err.Error())
	return message == "repository store is not configured" || message == "repository manager is not configured"
}

func logicalPathDepth(logicalPath string) int {
	trimmed := strings.Trim(strings.TrimSpace(logicalPath), "/")
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "/"))
}

func requireOrchestrator(deps Dependencies) (orchestratordomain.Orchestrator, error) {
	if deps.Orchestrator == nil {
		return nil, faults.NewValidationError("orchestrator is not configured", nil)
	}
	return deps.Orchestrator, nil
}
