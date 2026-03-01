package operator

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	readapp "github.com/crmarques/declarest/internal/app/resource/read"
	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/secrets"
)

const (
	SourceRepository   = readapp.SourceRepository
	SourceRemoteServer = readapp.SourceRemoteServer
)

type Dependencies struct {
	Orchestrator orchestrator.Orchestrator
	Contexts     config.ContextService
	Metadata     metadata.MetadataService
	Secrets      secrets.SecretProvider
}

type ReconcileRequest struct {
	LogicalPath  string
	Source       string
	ContextName  string
	ShowSecrets  bool
	ShowMetadata bool
}

type ReconcileResult struct {
	LogicalPath string   `json:"logicalPath" yaml:"logicalPath"`
	Output      any      `json:"output" yaml:"output"`
	TextLines   []string `json:"textLines,omitempty" yaml:"textLines,omitempty"`
}

func ReconcileOnce(ctx context.Context, deps Dependencies, req ReconcileRequest) (ReconcileResult, error) {
	logicalPath := strings.TrimSpace(req.LogicalPath)
	if logicalPath == "" {
		return ReconcileResult{}, validationError("logical path is required", nil)
	}

	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = SourceRemoteServer
	}

	explicitCollectionTarget := logicalPath != "/" && strings.HasSuffix(logicalPath, "/")

	result, err := readapp.Execute(ctx, readapp.Dependencies{
		Orchestrator: deps.Orchestrator,
		Contexts:     deps.Contexts,
		Metadata:     deps.Metadata,
		Secrets:      deps.Secrets,
	}, readapp.Request{
		LogicalPath:              logicalPath,
		Source:                   source,
		ExplicitCollectionTarget: explicitCollectionTarget,
		ShowSecrets:              req.ShowSecrets,
		ShowMetadata:             req.ShowMetadata,
		ContextName:              req.ContextName,
	})
	if err != nil {
		return ReconcileResult{}, err
	}

	return ReconcileResult{
		LogicalPath: logicalPath,
		Output:      result.OutputValue,
		TextLines:   result.TextLines,
	}, nil
}

func validationError(message string, cause error) error {
	return faults.NewTypedError(faults.ValidationError, message, cause)
}
