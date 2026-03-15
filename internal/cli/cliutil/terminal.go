// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cliutil

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
