package input

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

// DecodeOptionalMutationPayloadInput decodes explicit payload input for
// resource mutation commands. It supports file paths, stdin ("-"), inline
// JSON/YAML, and JSON Pointer assignment shorthand like "/a=b,/c=d,/e/f=g".
func DecodeOptionalMutationPayloadInput(
	command *cobra.Command,
	flags cliutil.InputFlags,
) (resource.Content, bool, error) {
	payloadArg := strings.TrimSpace(flags.Payload)
	if payloadArg == "" || payloadArg == "-" {
		return DecodeOptionalPayloadInput(command, flags)
	}
	if payloadArgLooksLikeExistingFile(payloadArg) {
		return DecodeOptionalPayloadInput(command, flags)
	}

	stdinData, err := cliutil.ReadOptionalInput(command, cliutil.InputFlags{})
	if err != nil {
		return resource.Content{}, false, err
	}
	if len(stdinData) > 0 {
		return resource.Content{}, false, cliutil.ValidationError("flag --payload cannot be combined with stdin input", nil)
	}

	if cliutil.IsBinaryInputFormat(flags.ContentType) {
		return resource.Content{}, false, cliutil.ValidationError("binary payload input requires --payload <path|-> or stdin", nil)
	}

	structuredAssignmentsAllowed, err := allowsStructuredAssignmentInput(flags.ContentType)
	if err != nil {
		return resource.Content{}, false, err
	}
	if structuredAssignmentsAllowed {
		if objectValue, err := cliutil.ParsePointerAssignmentsObject(payloadArg); err == nil {
			payloadType := assignmentPayloadType(flags.ContentType)
			return resource.Content{
				Value:      objectValue,
				Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: payloadType}),
			}, true, nil
		}

		if cliutil.IsDotNotationAssignment(payloadArg) {
			objectValue, err := cliutil.ParseDotNotationAssignmentsObject(payloadArg)
			if err != nil {
				return resource.Content{}, false, err
			}
			payloadType := assignmentPayloadType(flags.ContentType)
			return resource.Content{
				Value:      objectValue,
				Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: payloadType}),
			}, true, nil
		}
	}

	if payloadArgLooksLikeFilePath(payloadArg) {
		_, readErr := cliutil.ReadInput(command, flags)
		if readErr != nil {
			return resource.Content{}, false, readErr
		}
		return resource.Content{}, false, cliutil.ValidationError("invalid payload input", nil)
	}

	if !mutationPayloadAllowsInlineLiteral(flags.ContentType, payloadArg) {
		return resource.Content{}, false, cliutil.ValidationError("invalid payload input", nil)
	}

	if value, err := cliutil.DecodeResourceContentInputData([]byte(payloadArg), flags.ContentType, ""); err == nil {
		return value, true, nil
	}

	return resource.Content{}, false, cliutil.ValidationError("invalid payload input", nil)
}

func payloadArgLooksLikeExistingFile(value string) bool {
	info, err := os.Stat(value)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func payloadArgLooksLikeFilePath(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || trimmed == "-" {
		return false
	}
	if filepath.IsAbs(trimmed) {
		return true
	}
	if strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, "~") {
		return true
	}
	if strings.ContainsAny(trimmed, `/\`) {
		return true
	}
	return filepath.Ext(trimmed) != ""
}

// assignmentPayloadType returns the payload type to use for key=value
// assignment payloads based on the --content-type flag. When no content type is
// specified the default is JSON. Callers must validate the content type with
// allowsStructuredAssignmentInput before calling this function.
func assignmentPayloadType(contentType string) string {
	trimmed := strings.TrimSpace(contentType)
	if trimmed == "" {
		return resource.PayloadTypeJSON
	}
	descriptor, ok := resource.PayloadDescriptorForContentType(trimmed)
	if !ok {
		return resource.PayloadTypeJSON
	}
	return descriptor.PayloadType
}

func allowsStructuredAssignmentInput(contentType string) (bool, error) {
	trimmed := strings.TrimSpace(contentType)
	if trimmed == "" {
		return true, nil
	}

	descriptor, ok := resource.PayloadDescriptorForContentType(trimmed)
	if !ok {
		return false, cliutil.ValidationError(
			"invalid input content type: use json, yaml, xml, hcl, ini, properties, text, txt, binary, or a supported media type",
			nil,
		)
	}
	return resource.IsStructuredPayloadType(descriptor.PayloadType), nil
}

func mutationPayloadAllowsInlineLiteral(contentType string, payloadArg string) bool {
	trimmedContentType := strings.TrimSpace(contentType)
	if trimmedContentType != "" {
		descriptor, ok := resource.PayloadDescriptorForContentType(trimmedContentType)
		return ok && !resource.IsBinaryPayloadType(descriptor.PayloadType)
	}
	return resource.StructuredLookingPayload([]byte(payloadArg))
}
