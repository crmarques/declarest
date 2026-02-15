package common

import (
	"io"
	"os"

	"github.com/spf13/cobra"
)

func IsInteractiveTerminal(command *cobra.Command) bool {
	in, inInfo, ok := fileFromReader(command.InOrStdin())
	if !ok || in == nil || inInfo == nil {
		return false
	}
	out, outInfo, ok := fileFromWriter(command.OutOrStdout())
	if !ok || out == nil || outInfo == nil {
		return false
	}

	return (inInfo.Mode()&os.ModeCharDevice) != 0 && (outInfo.Mode()&os.ModeCharDevice) != 0
}

func HasPipedInput(command *cobra.Command) bool {
	_, info, ok := fileFromReader(command.InOrStdin())
	if !ok || info == nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) == 0
}

func fileFromReader(reader io.Reader) (*os.File, os.FileInfo, bool) {
	file, ok := reader.(*os.File)
	if !ok {
		return nil, nil, false
	}
	info, err := file.Stat()
	if err != nil {
		return nil, nil, false
	}
	return file, info, true
}

func fileFromWriter(writer io.Writer) (*os.File, os.FileInfo, bool) {
	file, ok := writer.(*os.File)
	if !ok {
		return nil, nil, false
	}
	info, err := file.Stat()
	if err != nil {
		return nil, nil, false
	}
	return file, info, true
}
