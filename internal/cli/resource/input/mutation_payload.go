package input

import (
	"os"
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

	stdinData, err := cliutil.ReadOptionalInput(command, cliutil.InputFlags{})
	if err != nil {
		return resource.Content{}, false, err
	}
	if len(stdinData) > 0 {
		return resource.Content{}, false, cliutil.ValidationError("flag --payload cannot be combined with stdin input", nil)
	}

	if payloadArgLooksLikeExistingFile(payloadArg) {
		return DecodeOptionalPayloadInput(command, flags)
	}
	if cliutil.IsBinaryInputFormat(flags.ContentType) {
		return resource.Content{}, false, cliutil.ValidationError("binary payload input requires --payload <path|-> or stdin", nil)
	}

	if objectValue, err := cliutil.ParsePointerAssignmentsObject(payloadArg); err == nil {
		return resource.Content{
			Value:      objectValue,
			Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
		}, true, nil
	}

	if value, err := cliutil.DecodeResourceContentInputData([]byte(payloadArg), flags.ContentType, ""); err == nil {
		return value, true, nil
	}

	// Preserve the existing missing-file behavior when the input looks like a
	// path but does not exist and also does not parse as supported inline input.
	_, readErr := cliutil.ReadInput(command, flags)
	if readErr != nil {
		return resource.Content{}, false, readErr
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
