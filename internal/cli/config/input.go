package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

type contextImportInputKind string

const (
	contextImportInputContext contextImportInputKind = "context"
	contextImportInputCatalog contextImportInputKind = "catalog"
)

type contextImportInput struct {
	Kind    contextImportInputKind
	Context configdomain.Context
	Catalog configdomain.ContextCatalog
}

func decodeContextStrict(command *cobra.Command, flags cliutil.InputFlags) (configdomain.Context, error) {
	data, err := cliutil.ReadInput(command, flags)
	if err != nil {
		return configdomain.Context{}, err
	}

	return decodeContextStrictFromData(data, flags.Format)
}

func decodeContextImportInputStrict(command *cobra.Command, flags cliutil.InputFlags) (contextImportInput, error) {
	data, err := cliutil.ReadInput(command, flags)
	if err != nil {
		return contextImportInput{}, err
	}

	decodedContext, contextErr := decodeContextStrictFromData(data, flags.Format)
	if contextErr == nil {
		return contextImportInput{
			Kind:    contextImportInputContext,
			Context: decodedContext,
		}, nil
	}

	decodedCatalog, catalogErr := decodeContextCatalogStrictFromData(data, flags.Format)
	if catalogErr == nil {
		return contextImportInput{
			Kind:    contextImportInputCatalog,
			Catalog: decodedCatalog,
		}, nil
	}

	return contextImportInput{}, cliutil.ValidationError(
		"input must be a context object or a context catalog",
		errors.Join(contextErr, catalogErr),
	)
}

func decodeContextStrictFromData(data []byte, format string) (configdomain.Context, error) {
	var output configdomain.Context
	if err := decodeInputStrict(data, format, &output); err != nil {
		return configdomain.Context{}, err
	}
	return output, nil
}

func decodeContextCatalogStrictFromData(data []byte, format string) (configdomain.ContextCatalog, error) {
	var output configdomain.ContextCatalog
	if err := decodeInputStrict(data, format, &output); err != nil {
		return configdomain.ContextCatalog{}, err
	}
	return output, nil
}

func decodeInputStrict(data []byte, format string, output any) error {
	switch format {
	case "", cliutil.OutputJSON:
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(output); err != nil {
			return cliutil.ValidationError("invalid json input", err)
		}

		var extra json.RawMessage
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			if err == nil {
				return cliutil.ValidationError("invalid json input", errors.New("multiple JSON values are not supported"))
			}
			return cliutil.ValidationError("invalid json input", err)
		}

	case cliutil.OutputYAML:
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(output); err != nil {
			return cliutil.ValidationError("invalid yaml input", err)
		}

		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			if err == nil {
				return cliutil.ValidationError("invalid yaml input", errors.New("multiple YAML documents are not supported"))
			}
			return cliutil.ValidationError("invalid yaml input", err)
		}

	default:
		return cliutil.ValidationError("invalid input format: use json or yaml", nil)
	}

	return nil
}
