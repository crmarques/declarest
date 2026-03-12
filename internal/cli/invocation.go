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
