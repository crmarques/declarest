package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func decodeContextStrict(command *cobra.Command, flags common.InputFlags) (configdomain.Context, error) {
	var output configdomain.Context

	data, err := common.ReadInput(command, flags)
	if err != nil {
		return output, err
	}

	switch flags.Format {
	case "", common.OutputJSON:
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&output); err != nil {
			return output, common.ValidationError("invalid json input", err)
		}

		var extra json.RawMessage
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			if err == nil {
				return output, common.ValidationError("invalid json input", errors.New("multiple JSON values are not supported"))
			}
			return output, common.ValidationError("invalid json input", err)
		}

	case common.OutputYAML:
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		decoder.KnownFields(true)
		if err := decoder.Decode(&output); err != nil {
			return output, common.ValidationError("invalid yaml input", err)
		}

		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			if err == nil {
				return output, common.ValidationError("invalid yaml input", errors.New("multiple YAML documents are not supported"))
			}
			return output, common.ValidationError("invalid yaml input", err)
		}

	default:
		return output, common.ValidationError("invalid input format: use json or yaml", nil)
	}

	return output, nil
}
