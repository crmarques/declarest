package request

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/faults"
	mutateapp "github.com/crmarques/declarest/internal/app/resource/mutate"
	"github.com/crmarques/declarest/managedserver"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
)

type Dependencies struct {
	Orchestrator orchestratordomain.Orchestrator
}

type Request struct {
	Method         string
	LogicalPath    string
	Body           resource.Content
	Headers        map[string]string
	Accept         string
	ContentType    string
	ResolveTargets bool
	Recursive      bool
}

type Result struct {
	Values []resource.Content
}

func Execute(ctx context.Context, deps Dependencies, req Request) (Result, error) {
	orchestratorService, err := requireOrchestrator(deps)
	if err != nil {
		return Result{}, err
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	baseSpec := managedserver.RequestSpec{
		Method:      method,
		Path:        req.LogicalPath,
		Headers:     cloneStringMap(req.Headers),
		Accept:      req.Accept,
		ContentType: req.ContentType,
		Body:        req.Body,
	}
	if !req.ResolveTargets {
		value, err := orchestratorService.Request(ctx, baseSpec)
		if err != nil {
			return Result{}, err
		}
		return Result{Values: []resource.Content{value}}, nil
	}

	targets, err := mutateapp.ListLocalTargetsOrFallbackPath(ctx, orchestratorService, req.LogicalPath, req.Recursive)
	if err != nil {
		return Result{}, err
	}

	results := make([]resource.Content, 0, len(targets))
	for _, target := range targets {
		spec := baseSpec
		spec.Path = target.LogicalPath
		value, err := orchestratorService.Request(ctx, spec)
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

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
