package cmd

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func newVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "version",
		GroupID: groupUtility,
		Short:   "Print the DeclaREST CLI version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), formatVersion())
			return nil
		},
	}
	return cmd
}

func formatVersion() string {
	version := strings.TrimSpace(Version)
	if version == "" {
		version = "dev"
	}
	commit := strings.TrimSpace(Commit)
	if commit == "" {
		commit = "none"
	}
	date := strings.TrimSpace(Date)
	if date == "" {
		date = "unknown"
	}
	return fmt.Sprintf("declarest %s (%s, %s) %s", version, commit, date, runtime.Version())
}
