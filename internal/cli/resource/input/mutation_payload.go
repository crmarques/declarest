package input

import (
	"os"
	"strings"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

// DecodeOptionalMutationPayloadInput decodes explicit payload input for
// resource mutation commands. It supports file paths, stdin ("-"), inline
// JSON/YAML, and dotted assignment shorthand like "a=b,c=d,e.f=g".
func DecodeOptionalMutationPayloadInput(
	command *cobra.Command,
	flags common.InputFlags,
) (resource.Value, bool, error) {
	payloadArg := strings.TrimSpace(flags.Payload)
	if payloadArg == "" || payloadArg == "-" {
		return DecodeOptionalPayloadInput(command, flags)
	}

	stdinData, err := common.ReadOptionalInput(command, common.InputFlags{})
	if err != nil {
		return nil, false, err
	}
	if len(stdinData) > 0 {
		return nil, false, common.ValidationError("flag --payload cannot be combined with stdin input", nil)
	}

	if payloadArgLooksLikeExistingFile(payloadArg) {
		return DecodeOptionalPayloadInput(command, flags)
	}

	if value, err := common.DecodeInputData[resource.Value]([]byte(payloadArg), flags.Format); err == nil {
		return value, true, nil
	}

	if objectValue, err := common.ParseDottedAssignmentsObject(payloadArg); err == nil {
		return objectValue, true, nil
	}

	// Preserve the existing missing-file behavior when the input looks like a
	// path but does not exist and also does not parse as supported inline input.
	_, readErr := common.ReadInput(command, flags)
	if readErr != nil {
		return nil, false, readErr
	}

	return nil, false, common.ValidationError("invalid payload input", nil)
}

func payloadArgLooksLikeExistingFile(value string) bool {
	info, err := os.Stat(value)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
