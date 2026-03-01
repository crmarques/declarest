package operator

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/faults"
	readapp "github.com/crmarques/declarest/internal/app/resource/read"
)

const (
	SourceRepository   = readapp.SourceRepository
	SourceRemoteServer = readapp.SourceRemoteServer
)

// Dependencies matches readapp.Dependencies; use a type alias to avoid
// duplicating the same struct and the manual field-by-field mapping.
type Dependencies = readapp.Dependencies

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

	result, err := readapp.Execute(ctx, deps, readapp.Request{
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
