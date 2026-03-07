package input

import (
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

// DecodeOptionalPayloadInput decodes a payload value from --payload or stdin.
// It returns hasInput=false when no input source was provided.
func DecodeOptionalPayloadInput(
	command *cobra.Command,
	flags cliutil.InputFlags,
) (resource.Value, bool, error) {
	data, err := cliutil.ReadOptionalInput(command, flags)
	if err != nil {
		return nil, false, err
	}
	if data == nil {
		return nil, false, nil
	}
	value, err := cliutil.DecodeResourceValueInputData(data, flags.Format)
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

// DecodeRequiredPayloadInput decodes a required payload value from --payload or stdin.
func DecodeRequiredPayloadInput(
	command *cobra.Command,
	flags cliutil.InputFlags,
) (resource.Value, error) {
	value, hasInput, err := DecodeOptionalPayloadInput(command, flags)
	if err != nil {
		return nil, err
	}
	if !hasInput {
		return nil, cliutil.ValidationError(cliutil.MissingInputMessage, nil)
	}
	return value, nil
}
