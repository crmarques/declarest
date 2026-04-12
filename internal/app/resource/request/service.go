// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package request

import (
	"context"
	"maps"
	"strings"

	appdeps "github.com/crmarques/declarest/internal/app/deps"
	mutateapp "github.com/crmarques/declarest/internal/app/resource/mutate"
	"github.com/crmarques/declarest/managedservice"
	"github.com/crmarques/declarest/resource"
)

type Dependencies = appdeps.Dependencies

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
	orchestratorService, err := appdeps.RequireOrchestrator(deps)
	if err != nil {
		return Result{}, err
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	baseSpec := managedservice.RequestSpec{
		Method:      method,
		Path:        req.LogicalPath,
		Headers:     maps.Clone(req.Headers),
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
