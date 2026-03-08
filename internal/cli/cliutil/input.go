package cliutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/crmarques/declarest/resource"
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

	return DecodeInputData[T](data, flags.ContentType, flags.Payload)
}

func DecodeInputData[T any](data []byte, contentType string, sourceName string) (T, error) {
	var output T

	switch resolveStructuredInputContentType(contentType, sourceName) {
	case OutputJSON:
		if err := json.Unmarshal(data, &output); err != nil {
			return output, ValidationError("invalid json input", err)
		}
	case OutputYAML:
		if err := yaml.Unmarshal(data, &output); err != nil {
			return output, ValidationError("invalid yaml input", err)
		}
	default:
		return output, ValidationError("invalid input content type: use json, yaml, application/json, or application/yaml", nil)
	}

	return output, nil
}

func DecodeResourceValueInputData(data []byte, contentType string) (resource.Value, error) {
	content, err := DecodeResourceContentInputData(data, contentType, "")
	if err != nil {
		return nil, err
	}
	return content.Value, nil
}

func DecodeResourceContentInputData(data []byte, contentType string, sourceName string) (resource.Content, error) {
	descriptor, err := resourceInputPayloadDescriptor(data, contentType, sourceName)
	if err != nil {
		return resource.Content{}, err
	}
	content, err := resource.DecodeContent(data, descriptor)
	if err != nil {
		return resource.Content{}, err
	}
	return content, nil
}

func IsBinaryInputFormat(contentType string) bool {
	descriptor, ok := resource.PayloadDescriptorForContentType(strings.TrimSpace(contentType))
	if !ok {
		return false
	}
	return descriptor.PayloadType == resource.PayloadTypeOctetStream
}

func resourceInputPayloadDescriptor(data []byte, contentType string, sourceName string) (resource.PayloadDescriptor, error) {
	if trimmed := strings.TrimSpace(contentType); trimmed != "" {
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{MediaType: trimmed}), nil
	}
	if descriptor, ok := resource.PayloadDescriptorForFileName(sourceName); ok {
		return descriptor, nil
	}
	if resource.StructuredLookingPayload(data) {
		return resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}), nil
	}
	return resource.DefaultOctetStreamDescriptor(), nil
}

func readInput(command *cobra.Command, flags InputFlags, required bool) ([]byte, error) {
	if flags.Payload != "" && flags.Payload != stdinFileIndicator {
		file, err := os.Open(flags.Payload)
		if err != nil {
			return nil, err
		}
		defer func() {
			_ = file.Close()
		}()

		data, err := readAllWithLimit(file, maxInputBytes)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 && IsBinaryInputFormat(flags.ContentType) {
			return data, nil
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
	if len(data) == 0 && IsBinaryInputFormat(flags.ContentType) {
		if required || flags.Payload == stdinFileIndicator {
			return data, nil
		}
		return nil, nil
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

func resourceInputPayloadType(contentType string) (string, error) {
	switch resolveStructuredInputContentType(contentType, "") {
	case OutputJSON:
		return resource.PayloadTypeJSON, nil
	case OutputYAML:
		return resource.PayloadTypeYAML, nil
	case resource.PayloadTypeXML:
		return resource.PayloadTypeXML, nil
	case resource.PayloadTypeHCL:
		return resource.PayloadTypeHCL, nil
	case resource.PayloadTypeINI:
		return resource.PayloadTypeINI, nil
	case resource.PayloadTypeProperties:
		return resource.PayloadTypeProperties, nil
	case resource.PayloadTypeText:
		return resource.PayloadTypeText, nil
	case resource.PayloadTypeBinary, resource.PayloadTypeOctetStream:
		return resource.PayloadTypeOctetStream, nil
	default:
		return "", ValidationError("invalid input content type", nil)
	}
}

func resolveStructuredInputContentType(contentType string, sourceName string) string {
	if descriptor, ok := resource.PayloadDescriptorForContentType(contentType); ok {
		return descriptor.PayloadType
	}
	if descriptor, ok := resource.PayloadDescriptorForFileName(sourceName); ok {
		return descriptor.PayloadType
	}
	return OutputJSON
}
