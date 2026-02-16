package cli

import (
	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
)

type Dependencies struct {
	Orchestrator orchestrator.Orchestrator
	Contexts     config.ContextService
	Repository   repository.ResourceRepository
	Metadata     metadata.MetadataService
	Secrets      secrets.SecretProvider
}

func (d Dependencies) commandDependencies() common.CommandDependencies {
	return common.CommandDependencies{
		Orchestrator: d.Orchestrator,
		Contexts:     d.Contexts,
		Repository:   d.Repository,
		Metadata:     d.Metadata,
		Secrets:      d.Secrets,
	}
}

func Execute(deps Dependencies) error {
	return NewRootCommand(deps).Execute()
}
