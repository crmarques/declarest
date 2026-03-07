package operator

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/faults"
	readapp "github.com/crmarques/declarest/internal/app/resource/read"
	"github.com/crmarques/declarest/resource"
)

const (
	SourceRepository    = readapp.SourceRepository
	SourceManagedServer = readapp.SourceManagedServer
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
		return ReconcileResult{}, faults.NewValidationError("logical path is required", nil)
	}
	parsedPath, err := resource.ParseRawPathWithOptions(logicalPath, resource.RawPathParseOptions{
		AllowMissingLeadingSlash: true,
	})
	if err != nil {
		return ReconcileResult{}, err
	}
	normalizedPath, err := resource.NormalizeLogicalPath(parsedPath.Normalized)
	if err != nil {
		return ReconcileResult{}, err
	}

	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = SourceManagedServer
	}

	explicitCollectionTarget := parsedPath.ExplicitCollectionTarget

	result, err := readapp.Execute(ctx, deps, readapp.Request{
		LogicalPath:              normalizedPath,
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
		LogicalPath: normalizedPath,
		Output:      result.OutputValue,
		TextLines:   result.TextLines,
	}, nil
}
