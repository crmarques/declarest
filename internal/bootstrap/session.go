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

package bootstrap

import (
	"context"

	"github.com/crmarques/declarest/config"
	configfile "github.com/crmarques/declarest/internal/providers/config/file"
)

func NewContextService(opts BootstrapConfig) config.ContextService {
	return configfile.NewService(opts.ContextCatalogPath)
}

func NewSession(opts BootstrapConfig, selection config.ContextSelection) (Session, error) {
	contextService := NewContextService(opts)
	orch, err := buildOrchestrator(context.Background(), contextService, selection)
	if err != nil {
		return Session{}, err
	}

	return Session{
		Contexts:     contextService,
		Orchestrator: orch,
		Services:     orch,
	}, nil
}

func NewSessionFromResolvedContext(resolvedContext config.Context) (Session, error) {
	orch, err := buildOrchestratorFromResolvedContext(context.Background(), resolvedContext)
	if err != nil {
		return Session{}, err
	}
	return Session{
		Contexts:     nil,
		Orchestrator: orch,
		Services:     orch,
	}, nil
}
