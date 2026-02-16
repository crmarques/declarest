package resource

import (
	"fmt"
	"io"

	"github.com/crmarques/declarest/internal/cli/common"
	debugctx "github.com/crmarques/declarest/internal/support/debug"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

const (
	sourceLocal  = "local"
	sourceRemote = "remote"
)

func NewCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "resource",
		Short: "Manage resources",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newGetCommand(deps, globalFlags),
		newSaveCommand(deps),
		newApplyCommand(deps, globalFlags),
		newCreateCommand(deps, globalFlags),
		newUpdateCommand(deps, globalFlags),
		newDeleteCommand(deps),
		newDiffCommand(deps, globalFlags),
		newListCommand(deps, globalFlags),
		newExplainCommand(deps, globalFlags),
		newTemplateCommand(deps, globalFlags),
	)

	return command
}

func newGetCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "get [path]",
		Short: "Read a resource",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			debugctx.Printf(command.Context(), "resource get requested path=%q", resolvedPath)

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			reconciler, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			value, err := reconciler.Get(command.Context(), resolvedPath)
			if err != nil {
				debugctx.Printf(command.Context(), "resource get failed path=%q error=%v", resolvedPath, err)
				return err
			}

			debugctx.Printf(command.Context(), "resource get succeeded path=%q value_type=%T", resolvedPath, value)

			return common.WriteOutput(command, outputFormat, value, func(w io.Writer, item resource.Value) error {
				_, writeErr := fmt.Fprintln(w, item)
				return writeErr
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	return command
}

func newSaveCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "save [path]",
		Short: "Save local resource value",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			reconciler, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}

			return reconciler.Save(command.Context(), resolvedPath, value)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.BindInputFlags(command, &input)
	return command
}

func newApplyCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "apply [path]",
		Short: "Apply local desired state",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			reconciler, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			item, err := reconciler.Apply(command.Context(), resolvedPath)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, item, func(w io.Writer, value resource.Resource) error {
				_, writeErr := fmt.Fprintln(w, value.LogicalPath)
				return writeErr
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	return command
}

func newCreateCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "create [path]",
		Short: "Create remote resource",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			reconciler, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			item, err := reconciler.Create(command.Context(), resolvedPath, value)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, item, func(w io.Writer, output resource.Resource) error {
				_, writeErr := fmt.Fprintln(w, output.LogicalPath)
				return writeErr
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.BindInputFlags(command, &input)
	return command
}

func newUpdateCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "update [path]",
		Short: "Update remote resource",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			reconciler, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			item, err := reconciler.Update(command.Context(), resolvedPath, value)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, item, func(w io.Writer, output resource.Resource) error {
				_, writeErr := fmt.Fprintln(w, output.LogicalPath)
				return writeErr
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.BindInputFlags(command, &input)
	return command
}

func newDeleteCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var force bool
	var recursive bool

	command := &cobra.Command{
		Use:   "delete [path]",
		Short: "Delete a resource",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if !force {
				return command.Help()
			}
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			reconciler, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			return reconciler.Delete(command.Context(), resolvedPath, orchestratordomain.DeletePolicy{Recursive: recursive})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	command.Flags().BoolVarP(&force, "force", "y", false, "confirm deletion")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "delete recursively")
	return command
}

func newDiffCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "diff [path]",
		Short: "Compare local and remote state",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			reconciler, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			items, err := reconciler.Diff(command.Context(), resolvedPath)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, items, func(w io.Writer, value []resource.DiffEntry) error {
				for _, item := range value {
					if _, writeErr := fmt.Fprintf(w, "%s %s\n", item.Operation, item.Path); writeErr != nil {
						return writeErr
					}
				}
				return nil
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	return command
}

func newListCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var source string
	var recursive bool

	command := &cobra.Command{
		Use:   "list [path]",
		Short: "List resources",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			reconciler, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}

			var items []resource.Resource
			switch source {
			case sourceLocal:
				items, err = reconciler.ListLocal(command.Context(), resolvedPath, orchestratordomain.ListPolicy{Recursive: recursive})
			case sourceRemote:
				items, err = reconciler.ListRemote(command.Context(), resolvedPath, orchestratordomain.ListPolicy{Recursive: recursive})
			default:
				return common.ValidationError("invalid source: use local or remote", nil)
			}
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, items, func(w io.Writer, value []resource.Resource) error {
				for _, item := range value {
					if _, writeErr := fmt.Fprintln(w, item.LogicalPath); writeErr != nil {
						return writeErr
					}
				}
				return nil
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	command.Flags().StringVarP(&source, "source", "s", sourceLocal, "list source: local|remote")
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "list recursively")
	return command
}

func newExplainCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string

	command := &cobra.Command{
		Use:   "explain [path]",
		Short: "Explain planned changes",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			reconciler, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			items, err := reconciler.Explain(command.Context(), resolvedPath)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, items, func(w io.Writer, value []resource.DiffEntry) error {
				for _, item := range value {
					if _, writeErr := fmt.Fprintf(w, "%s %s\n", item.Operation, item.Path); writeErr != nil {
						return writeErr
					}
				}
				return nil
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	return command
}

func newTemplateCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "template [path]",
		Short: "Render payload templates",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}
			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			reconciler, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			templated, err := reconciler.Template(command.Context(), resolvedPath, value)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, templated, func(w io.Writer, item resource.Value) error {
				_, writeErr := fmt.Fprintln(w, item)
				return writeErr
			})
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.BindInputFlags(command, &input)
	return command
}
