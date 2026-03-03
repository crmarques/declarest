package input

import (
	"errors"

	"github.com/crmarques/declarest/faults"
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
	value, err := cliutil.DecodeInput[resource.Value](command, flags)
	if err == nil {
		return value, true, nil
	}
	if isMissingInputError(err) {
		return nil, false, nil
	}
	return nil, false, err
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

func isMissingInputError(err error) bool {
	if err == nil {
		return false
	}

	var typedErr *faults.TypedError
	return errors.As(err, &typedErr) &&
		typedErr.Category == faults.ValidationError &&
		typedErr.Message == cliutil.MissingInputMessage
}
