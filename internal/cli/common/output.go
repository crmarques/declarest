package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

const (
	OutputAuto = "auto"
	OutputText = "text"
	OutputJSON = "json"
	OutputYAML = "yaml"
)

func ValidateOutputFormat(format string) error {
	switch format {
	case OutputAuto, OutputText, OutputJSON, OutputYAML:
		return nil
	default:
		return ValidationError("invalid output format: use auto, text, json, or yaml", nil)
	}
}

func ResolveContextOutputFormat(ctx context.Context, deps CommandDependencies, globalFlags *GlobalFlags) (string, error) {
	if globalFlags == nil || globalFlags.Output == "" {
		return OutputJSON, nil
	}
	if globalFlags.Output != OutputAuto {
		return globalFlags.Output, nil
	}
	if deps.Contexts == nil {
		return OutputJSON, nil
	}

	resolvedContext, err := deps.Contexts.ResolveContext(ctx, configdomain.ContextSelection{Name: globalFlags.Context})
	if err != nil {
		return "", err
	}

	switch resolvedContext.Repository.ResourceFormat {
	case "", configdomain.ResourceFormatJSON:
		return OutputJSON, nil
	case configdomain.ResourceFormatYAML:
		return OutputYAML, nil
	default:
		return "", ValidationError("invalid repository resource format in context", nil)
	}
}

func WriteOutput[T any](command *cobra.Command, format string, value T, renderText func(io.Writer, T) error) error {
	switch format {
	case OutputAuto, OutputText:
		if renderText != nil {
			return renderText(command.OutOrStdout(), value)
		}
		_, err := fmt.Fprintln(command.OutOrStdout(), value)
		return err
	case OutputJSON:
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.OutOrStdout(), string(encoded))
		return err
	case OutputYAML:
		encoded, err := yaml.Marshal(value)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(command.OutOrStdout(), string(encoded))
		return err
	default:
		return ValidationError("invalid output format: use auto, text, json, or yaml", nil)
	}
}

func WriteText(command *cobra.Command, format string, text string) error {
	return WriteOutput(command, format, text, func(w io.Writer, value string) error {
		_, err := fmt.Fprintln(w, value)
		return err
	})
}
