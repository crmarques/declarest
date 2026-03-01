package request

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/faults"
	mutateapp "github.com/crmarques/declarest/internal/app/resource/mutate"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
)

type Dependencies struct {
	Orchestrator orchestratordomain.Orchestrator
}

type Request struct {
	Method         string
	LogicalPath    string
	Body           resource.Value
	ResolveTargets bool
	Recursive      bool
}

type Result struct {
	Values []resource.Value
}

func Execute(ctx context.Context, deps Dependencies, req Request) (Result, error) {
	orchestratorService, err := requireOrchestrator(deps)
	if err != nil {
		return Result{}, err
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if !req.ResolveTargets {
		value, err := orchestratorService.Request(ctx, method, req.LogicalPath, req.Body)
		if err != nil {
			return Result{}, err
		}
		return Result{Values: []resource.Value{value}}, nil
	}

	targets, err := mutateapp.ListLocalTargetsOrFallbackPath(ctx, orchestratorService, req.LogicalPath, req.Recursive)
	if err != nil {
		return Result{}, err
	}

	results := make([]resource.Value, 0, len(targets))
	for _, target := range targets {
		value, err := orchestratorService.Request(ctx, method, target.LogicalPath, req.Body)
		if err != nil {
			return Result{}, err
		}
		results = append(results, value)
	}

	return Result{Values: results}, nil
}

func requireOrchestrator(deps Dependencies) (orchestratordomain.Orchestrator, error) {
	if deps.Orchestrator == nil {
		return nil, faults.NewTypedError(faults.ValidationError, "orchestrator is not configured", nil)
	}
	return deps.Orchestrator, nil
}
