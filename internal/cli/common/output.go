package common

import (
	"fmt"

	"github.com/spf13/cobra"
)

const PlaceholderMessage = "to be implemented"

func RunPlaceholder(command *cobra.Command, _ []string) error {
	_, err := fmt.Fprintln(command.OutOrStdout(), PlaceholderMessage)
	return err
}

func NewPlaceholderCommand(use string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		RunE:  RunPlaceholder,
		Args:  cobra.ArbitraryArgs,
		Short: PlaceholderMessage,
	}
}
