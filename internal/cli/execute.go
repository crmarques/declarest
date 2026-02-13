package cli

import "github.com/crmarques/declarest/internal/cli/common"

func Execute(deps common.CommandWiring) error {
	return NewRootCommand(deps).Execute()
}
