package common

import (
	"bytes"
	"encoding/json"
	"io"
	"os"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func ReadInput(command *cobra.Command, flags InputFlags) ([]byte, error) {
	return readInput(command, flags, true)
}

func ReadOptionalInput(command *cobra.Command, flags InputFlags) ([]byte, error) {
	return readInput(command, flags, false)
}

func DecodeInput[T any](command *cobra.Command, flags InputFlags) (T, error) {
	var output T

	data, err := ReadInput(command, flags)
	if err != nil {
		return output, err
	}

	return DecodeInputData[T](data, flags.Format)
}

func DecodeInputData[T any](data []byte, format string) (T, error) {
	var output T

	switch format {
	case "", OutputJSON:
		if err := json.Unmarshal(data, &output); err != nil {
			return output, ValidationError("invalid json input", err)
		}
	case OutputYAML:
		if err := yaml.Unmarshal(data, &output); err != nil {
			return output, ValidationError("invalid yaml input", err)
		}
	default:
		return output, ValidationError("invalid input format: use json or yaml", nil)
	}

	return output, nil
}

func readInput(command *cobra.Command, flags InputFlags, required bool) ([]byte, error) {
	if flags.File != "" {
		data, err := os.ReadFile(flags.File)
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(data)) == 0 {
			return nil, ValidationError("input is empty", nil)
		}
		return data, nil
	}

	inputReader := command.InOrStdin()
	if stdinFile, ok := inputReader.(*os.File); ok {
		info, err := stdinFile.Stat()
		if err == nil && (info.Mode()&os.ModeCharDevice) != 0 {
			if required {
				return nil, ValidationError("input is required: provide --file or stdin", nil)
			}
			return nil, nil
		}
	}

	data, err := io.ReadAll(inputReader)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		if required {
			return nil, ValidationError("input is required: provide --file or stdin", nil)
		}
		return nil, nil
	}

	return data, nil
}
