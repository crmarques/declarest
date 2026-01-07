package cmd

import (
	"fmt"
	"text/tabwriter"

	ctx "declarest/internal/context"

	"github.com/spf13/cobra"
)

func newConfigEnvCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "env",
		Args:    cobra.NoArgs,
		Short:   "Show DeclaREST environment overrides",
		Long:    "List the environment variables that influence where DeclaREST stores context state and the resolved values.",
		Example: "  declarest config env",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigEnv(cmd)
		},
	}
	return cmd
}

func runConfigEnv(cmd *cobra.Command) error {
	dirInfo, err := ctx.ConfigDirPathInfo()
	if err != nil {
		return err
	}
	fileInfo, err := ctx.ConfigFilePathInfo()
	if err != nil {
		return err
	}

	entries := []envEntry{
		{
			Name:   ctx.ConfigDirEnvVar,
			Value:  dirInfo.Path,
			Source: configDirSource(dirInfo),
		},
		{
			Name:   ctx.ConfigFileEnvVar,
			Value:  fileInfo.Path,
			Source: configFileSource(fileInfo, dirInfo),
		},
	}

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "NAME\tVALUE\tSOURCE")
	for _, entry := range entries {
		if _, err := fmt.Fprintf(writer, "%s\t%s\t%s\n", entry.Name, entry.Value, entry.Source); err != nil {
			return err
		}
	}
	return writer.Flush()
}

type envEntry struct {
	Name   string
	Value  string
	Source string
}

func configDirSource(info ctx.ConfigPathInfo) string {
	if info.FromEnv {
		return fmt.Sprintf("environment (%s)", ctx.ConfigDirEnvVar)
	}
	return "default (HOME/.declarest)"
}

func configFileSource(fileInfo, dirInfo ctx.ConfigPathInfo) string {
	if fileInfo.FromEnv {
		return fmt.Sprintf("environment (%s)", ctx.ConfigFileEnvVar)
	}
	if dirInfo.FromEnv {
		return fmt.Sprintf("default (derived from %s)", ctx.ConfigDirEnvVar)
	}
	return "default (HOME/.declarest/config)"
}
