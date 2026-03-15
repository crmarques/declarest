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

package completion

import (
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	"github.com/spf13/cobra"
)

func NewCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	_ = deps
	_ = globalFlags

	command := &cobra.Command{
		Use:   "completion",
		Short: "Generate shell completion scripts",
		Args:  cobra.NoArgs,
	}

	bashCommand := newBashCommand()
	zshCommand := newZshCommand()
	fishCommand := newFishCommand()
	powerShellCommand := newPowerShellCommand()
	commandmeta.MarkTextOnlyOutput(bashCommand)
	commandmeta.MarkTextOnlyOutput(zshCommand)
	commandmeta.MarkTextOnlyOutput(fishCommand)
	commandmeta.MarkTextOnlyOutput(powerShellCommand)

	command.AddCommand(
		bashCommand,
		zshCommand,
		fishCommand,
		powerShellCommand,
	)

	return command
}
