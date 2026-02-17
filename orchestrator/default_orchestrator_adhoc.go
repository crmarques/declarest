package orchestrator

import (
	"context"

	debugctx "github.com/crmarques/declarest/internal/support/debug"
	"github.com/crmarques/declarest/resource"
)

func (r *DefaultOrchestrator) AdHoc(
	ctx context.Context,
	method string,
	endpointPath string,
	body resource.Value,
) (resource.Value, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}

	debugctx.Printf(
		ctx,
		"orchestrator ad-hoc request method=%q path=%q has_body=%t",
		method,
		endpointPath,
		body != nil,
	)

	value, err := serverManager.AdHoc(ctx, method, endpointPath, body)
	if err != nil {
		debugctx.Printf(
			ctx,
			"orchestrator ad-hoc request failed method=%q path=%q error=%v",
			method,
			endpointPath,
			err,
		)
		return nil, err
	}

	debugctx.Printf(
		ctx,
		"orchestrator ad-hoc request succeeded method=%q path=%q value_type=%T",
		method,
		endpointPath,
		value,
	)
	return value, nil
}

func (r *DefaultOrchestrator) GetOpenAPISpec(ctx context.Context) (resource.Value, error) {
	serverManager, err := r.requireServer()
	if err != nil {
		return nil, err
	}
	return serverManager.GetOpenAPISpec(ctx)
}
