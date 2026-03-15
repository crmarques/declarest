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

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/resource"
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

	return decodeContextStrictFromData(data, flags.ContentType, flags.Payload)
}

func decodeContextImportInputStrict(command *cobra.Command, flags cliutil.InputFlags) (contextImportInput, error) {
	data, err := cliutil.ReadInput(command, flags)
	if err != nil {
		return contextImportInput{}, err
	}

	decodedContext, contextErr := decodeContextStrictFromData(data, flags.ContentType, flags.Payload)
	if contextErr == nil {
		return contextImportInput{
			Kind:    contextImportInputContext,
			Context: decodedContext,
		}, nil
	}

	decodedCatalog, catalogErr := decodeContextCatalogStrictFromData(data, flags.ContentType, flags.Payload)
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

func decodeContextStrictFromData(data []byte, contentType string, sourceName string) (configdomain.Context, error) {
	var output configdomain.Context
	if err := decodeInputStrict(data, contentType, sourceName, &output); err != nil {
		return configdomain.Context{}, err
	}
	return output, nil
}

func decodeContextCatalogStrictFromData(data []byte, contentType string, sourceName string) (configdomain.ContextCatalog, error) {
	var output configdomain.ContextCatalog
	if err := decodeInputStrict(data, contentType, sourceName, &output); err != nil {
		return configdomain.ContextCatalog{}, err
	}
	return output, nil
}

func decodeInputStrict(data []byte, contentType string, sourceName string, output any) error {
	switch resolveConfigInputPayloadType(contentType, sourceName) {
	case cliutil.OutputJSON:
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
		return cliutil.ValidationError("invalid input content type: use json or yaml", nil)
	}

	return nil
}

func resolveConfigInputPayloadType(contentType string, sourceName string) string {
	if normalized, err := normalizeConfigInputContentType(contentType); err == nil {
		if descriptor, ok := resource.PayloadDescriptorForContentType(normalized); ok {
			return descriptor.PayloadType
		}
	}
	if strings.TrimSpace(contentType) != "" {
		return ""
	}
	if descriptor, ok := resource.PayloadDescriptorForFileName(sourceName); ok {
		return descriptor.PayloadType
	}
	return cliutil.OutputJSON
}

func normalizeConfigInputContentType(contentType string) (string, error) {
	switch strings.TrimSpace(contentType) {
	case "":
		return "", nil
	case cliutil.OutputJSON:
		return "application/json", nil
	case cliutil.OutputYAML:
		return "application/yaml", nil
	default:
		return "", cliutil.ValidationError("invalid input content type: use json or yaml", nil)
	}
}
