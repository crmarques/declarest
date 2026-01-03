package cmd

import (
	"strings"

	"github.com/spf13/cobra"
)

const usageTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Available Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{range $group := .Groups}}

{{.Title}}{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if not .AllChildCommandsHaveGroup}}

Additional Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .LocalNonPersistentFlags.HasAvailableFlags}}

Flags:
{{.LocalNonPersistentFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if hasGlobalFlags .}}

Global Flags:
{{globalFlagUsages . | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicCommand}}
  {{rpad .CommandPath .CommandPathPadding}} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}
`

func configureUsage(cmd *cobra.Command) {
	if cmd == nil {
		return
	}
	cobra.AddTemplateFunc("globalFlagUsages", globalFlagUsages)
	cobra.AddTemplateFunc("hasGlobalFlags", hasGlobalFlags)

	cmd.SetUsageTemplate(usageTemplate)
	if cmd.PersistentFlags().Lookup("help") == nil {
		cmd.PersistentFlags().BoolP("help", "h", false, "help for this command")
		_ = cmd.PersistentFlags().SetAnnotation("help", cobra.FlagSetByCobraAnnotation, []string{"true"})
	}
}

func hasGlobalFlags(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if cmd.PersistentFlags().HasAvailableFlags() {
		return true
	}
	return cmd.InheritedFlags().HasAvailableFlags()
}

func globalFlagUsages(cmd *cobra.Command) string {
	if cmd == nil {
		return ""
	}
	var parts []string
	if cmd.PersistentFlags().HasAvailableFlags() {
		usage := strings.TrimRight(cmd.PersistentFlags().FlagUsages(), "\n")
		if usage != "" {
			parts = append(parts, usage)
		}
	}
	if cmd.InheritedFlags().HasAvailableFlags() {
		usage := strings.TrimRight(cmd.InheritedFlags().FlagUsages(), "\n")
		if usage != "" {
			parts = append(parts, usage)
		}
	}
	return strings.Join(parts, "\n")
}
