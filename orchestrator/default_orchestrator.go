package orchestrator

import (
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/secrets"
	"github.com/crmarques/declarest/server"
)

var _ Orchestrator = (*DefaultOrchestrator)(nil)

type DefaultOrchestrator struct {
	Repository repository.ResourceRepository
	Metadata   metadata.MetadataService
	Server     server.ResourceServer
	Secrets    secrets.SecretProvider
}
