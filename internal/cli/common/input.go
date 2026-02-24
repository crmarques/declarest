package common

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

const (
	stdinFileIndicator  = "-"
	MissingInputMessage = "input is required: provide --payload <path|-> or stdin"
	maxInputBytes       = 4 << 20
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
	if flags.Payload != "" && flags.Payload != stdinFileIndicator {
		file, err := os.Open(flags.Payload)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		data, err := readAllWithLimit(file, maxInputBytes)
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
				return nil, ValidationError(MissingInputMessage, nil)
			}
			return nil, nil
		}
	}

	data, err := readAllWithLimit(inputReader, maxInputBytes)
	if err != nil {
		return nil, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		if required {
			return nil, ValidationError(MissingInputMessage, nil)
		}
		return nil, nil
	}

	return data, nil
}

func readAllWithLimit(reader io.Reader, maxBytes int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, ValidationError("input exceeds maximum supported size", errors.New("input too large"))
	}
	return data, nil
}
