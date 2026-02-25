package resource

import (
	"context"
	"fmt"
	"strings"

	resourcesave "github.com/crmarques/declarest/internal/app/resource/save"
	"github.com/crmarques/declarest/internal/cli/common"
	resourcedomain "github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newCopyCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var pathFlag string
	var targetPathFlag string
	var overrideAttributes string
	var overwrite bool
	var commitMessageAppend string

	command := &cobra.Command{
		Use:   "copy [path] [target-path]",
		Short: "Copy a repository resource to another local path (repo-only)",
		Example: strings.Join([]string{
			"  declarest resource copy /a/b/c /x/y/z",
			"  declarest resource copy /a/b/c /x/y/z --overwrite",
			"  declarest resource copy /a/b/c /x/y/z --message ticket-123",
			"  declarest resource copy /a/b/c /x/y/z --override-attributes a=b,c=d,e.f.g=h",
			"  declarest resource copy --path /a/b/c --target-path /x/y/z --override-attributes a=b",
		}, "\n"),
		Args: cobra.MaximumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			originPath, targetPath, err := resolveCopyPathInputs(pathFlag, targetPathFlag, args)
			if err != nil {
				return err
			}
			cfg, err := resolveActiveResourceContext(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if err := ensureCleanGitWorktreeForAutoCommit(command.Context(), deps, cfg, "resource copy"); err != nil {
				return err
			}

			value, err := resolveCopySourceValue(command.Context(), deps, originPath)
			if err != nil {
				return err
			}

			value, err = applyCopyOverrideAttributes(value, overrideAttributes)
			if err != nil {
				return err
			}

			if err := validateExplicitMutationPayloadIdentity(
				command.Context(),
				command.CommandPath(),
				deps,
				targetPath,
				value,
			); err != nil {
				return err
			}

			if err := resourcesave.Execute(
				command.Context(),
				resourcesave.Dependencies{
					Orchestrator: deps.Orchestrator,
					Repository:   deps.ResourceStore,
					Metadata:     deps.Metadata,
					Secrets:      deps.Secrets,
				},
				targetPath,
				value,
				true,
				resourcesave.ExecuteOptions{
					AsOneResource: true,
					Force:         overwrite,
				},
			); err != nil {
				return err
			}

			commitMessage := fmt.Sprintf("declarest: copy resource %s to %s", originPath, targetPath)
			if command.Flags().Changed("message") {
				appendValue := strings.TrimSpace(commitMessageAppend)
				if appendValue == "" {
					return common.ValidationError("flag --message cannot be empty", nil)
				}
				commitMessage = commitMessage + " - " + appendValue
			}

			return commitAndMaybeAutoSyncRepository(
				command.Context(),
				deps,
				cfg,
				commitMessage,
			)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.Flags().StringVar(&targetPathFlag, "target-path", "", "destination resource path")
	_ = command.RegisterFlagCompletionFunc("target-path", func(
		cmd *cobra.Command,
		_ []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		return common.CompleteLogicalPaths(cmd, deps, toComplete)
	})
	command.Flags().BoolVar(&overwrite, "overwrite", false, "allow replacing the target resource when it already exists")
	command.Flags().BoolVar(&overwrite, "override", false, "legacy alias for --overwrite")
	_ = command.Flags().MarkHidden("override")
	command.Flags().StringVarP(&commitMessageAppend, "message", "m", "", "append text to the default git commit message (git repositories only)")
	command.Flags().StringVar(&overrideAttributes, "override-attributes", "", "comma-separated dotted key=value attribute overrides for the copied payload")
	command.Flags().StringVar(&overrideAttributes, "overrides", "", "deprecated alias for --override-attributes")
	_ = command.Flags().MarkHidden("overrides")
	command.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= 2 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return common.CompleteLogicalPaths(cmd, deps, toComplete)
	}

	return command
}

func resolveCopyPathInputs(pathFlag string, targetPathFlag string, args []string) (string, string, error) {
	originArgs := []string{}
	if len(args) > 0 {
		originArgs = args[:1]
	}
	originPath, err := common.ResolvePathInput(pathFlag, originArgs, true)
	if err != nil {
		return "", "", err
	}

	positionalTarget := ""
	if len(args) > 1 {
		positionalTarget = args[1]
	}
	if strings.TrimSpace(targetPathFlag) != "" && positionalTarget != "" && strings.TrimSpace(targetPathFlag) != strings.TrimSpace(positionalTarget) {
		return "", "", common.ValidationError("path mismatch between positional argument and --target-path", nil)
	}

	targetPath := strings.TrimSpace(targetPathFlag)
	if targetPath == "" {
		targetPath = strings.TrimSpace(positionalTarget)
	}
	if targetPath == "" {
		return "", "", common.ValidationError("target path is required", nil)
	}

	return originPath, targetPath, nil
}

func resolveCopySourceValue(
	ctx context.Context,
	deps common.CommandDependencies,
	originPath string,
) (resourcedomain.Value, error) {
	normalizedPath, err := resourcedomain.NormalizeLogicalPath(originPath)
	if err != nil {
		return nil, err
	}

	orchestratorService := deps.Orchestrator
	if orchestratorService == nil {
		repositoryService, err := common.RequireResourceStore(deps)
		if err != nil {
			return nil, err
		}
		return repositoryService.Get(ctx, normalizedPath)
	}
	return orchestratorService.GetLocal(ctx, normalizedPath)
}

func applyCopyOverrideAttributes(value resourcedomain.Value, overrideAttributes string) (resourcedomain.Value, error) {
	if strings.TrimSpace(overrideAttributes) == "" {
		return value, nil
	}

	normalized, err := resourcedomain.Normalize(value)
	if err != nil {
		return nil, err
	}
	payloadMap, ok := normalized.(map[string]any)
	if !ok {
		return nil, common.ValidationError("--override-attributes requires an object payload", nil)
	}
	if err := common.ApplyDottedAssignmentsObject(payloadMap, overrideAttributes); err != nil {
		return nil, err
	}
	return resourcedomain.Normalize(payloadMap)
}
