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

package cli

import (
	"strings"
)

type Invocation struct {
	CommandName              string
	ParentCommandName        string
	PositionalArgs           []string
	RequiresContextBootstrap bool
}

func ResolveRunnableInvocation(args []string) (Invocation, bool) {
	root := NewRootCommand(Dependencies{})
	command, remainingArgs, err := root.Find(args)
	if err != nil || command == nil || !command.Runnable() {
		return Invocation{}, false
	}

	if err := command.ParseFlags(remainingArgs); err != nil {
		return Invocation{}, false
	}
	positionalArgs := command.Flags().Args()
	if err := command.ValidateArgs(positionalArgs); err != nil {
		return Invocation{}, false
	}

	parentName := ""
	if parent := command.Parent(); parent != nil {
		parentName = strings.TrimSpace(parent.Name())
	}

	return Invocation{
		CommandName:              strings.TrimSpace(command.Name()),
		ParentCommandName:        parentName,
		PositionalArgs:           positionalArgs,
		RequiresContextBootstrap: RequiresContextBootstrap(command),
	}, true
}
