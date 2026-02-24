package orchestrator

import (
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/server"
)

var _ Orchestrator = (*DefaultOrchestrator)(nil)

type DefaultOrchestrator struct {
	Repository repository.ResourceStore
	Metadata   metadata.MetadataService
	Server     server.ResourceServer
	Secrets    secrets.SecretProvider

	resourceFormat string
}

func (r *DefaultOrchestrator) SetResourceFormat(format string) {
	if r == nil {
		return
	}
	r.resourceFormat = metadata.NormalizeResourceFormat(format)
}

func (r *DefaultOrchestrator) effectiveResourceFormat() string {
	if r == nil {
		return metadata.NormalizeResourceFormat("")
	}
	return metadata.NormalizeResourceFormat(r.resourceFormat)
}
