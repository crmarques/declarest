package resource

import (
	"context"
	"fmt"
	"strings"

	"github.com/crmarques/declarest/faults"
	resourcesave "github.com/crmarques/declarest/internal/app/resource/save"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	resourcedomain "github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func newCopyCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var targetPathFlag string
	var overrideAttributes string
	var force bool
	var commitMessage string

	command := &cobra.Command{
		Use:   "copy [path] [target-path]",
		Short: "Copy a repository resource to another local path (repo-only)",
		Example: strings.Join([]string{
			"  declarest resource copy /a/b/c /x/y/z",
			"  declarest resource copy /a/b/c /x/y/z --force",
			"  declarest resource copy /a/b/c /x/y/z --message ticket-123",
			"  declarest resource copy /a/b/c /x/y/z --override-attributes /a=b,/c=d,/e/f/g=h",
			"  declarest resource copy --path /a/b/c --target-path /x/y/z --override-attributes /a=b",
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
					Repository:   deps.Services.RepositoryStore(),
					Metadata:     deps.Services.MetadataService(),
					Secrets:      deps.Services.SecretProvider(),
				},
				targetPath,
				value,
				true,
				resourcesave.ExecuteOptions{
					AsOneResource: true,
					Force:         force,
				},
			); err != nil {
				return err
			}

			commitMessage, err = resolveRepositoryCommitMessage(
				command,
				fmt.Sprintf("declarest: copy resource %s to %s", originPath, targetPath),
				commitMessage,
			)
			if err != nil {
				return err
			}

			return commitAndMaybeAutoSyncRepository(
				command.Context(),
				deps,
				cfg,
				commitMessage,
			)
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.Flags().StringVar(&targetPathFlag, "target-path", "", "destination resource path")
	_ = command.RegisterFlagCompletionFunc("target-path", func(
		cmd *cobra.Command,
		_ []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		return cliutil.CompleteLogicalPaths(cmd, deps, toComplete)
	})
	command.Flags().BoolVar(&force, "force", false, "allow replacing the target resource when it already exists")
	bindRepositoryCommitMessageFlags(command, &commitMessage)
	command.Flags().StringVar(&overrideAttributes, "override-attributes", "", "comma-separated JSON-pointer=value attribute overrides for the copied payload")
	command.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) >= 2 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return cliutil.CompleteLogicalPaths(cmd, deps, toComplete)
	}

	return command
}

func resolveCopyPathInputs(pathFlag string, targetPathFlag string, args []string) (string, string, error) {
	originArgs := []string{}
	if len(args) > 0 {
		originArgs = args[:1]
	}
	originPath, err := cliutil.ResolvePathInput(pathFlag, originArgs, true)
	if err != nil {
		return "", "", err
	}

	positionalTarget := ""
	if len(args) > 1 {
		positionalTarget = args[1]
	}
	if strings.TrimSpace(targetPathFlag) != "" && positionalTarget != "" && strings.TrimSpace(targetPathFlag) != strings.TrimSpace(positionalTarget) {
		return "", "", cliutil.ValidationError("path mismatch between positional argument and --target-path", nil)
	}

	targetPath := strings.TrimSpace(targetPathFlag)
	if targetPath == "" {
		targetPath = strings.TrimSpace(positionalTarget)
	}
	if targetPath == "" {
		return "", "", cliutil.ValidationError("target path is required", nil)
	}

	return originPath, targetPath, nil
}

func resolveCopySourceValue(
	ctx context.Context,
	deps cliutil.CommandDependencies,
	originPath string,
) (resourcedomain.Content, error) {
	normalizedPath, err := resourcedomain.NormalizeLogicalPath(originPath)
	if err != nil {
		return resourcedomain.Content{}, err
	}

	orchestratorService := deps.Orchestrator
	if orchestratorService == nil {
		repositoryService, err := cliutil.RequireResourceStore(deps)
		if err != nil {
			return resourcedomain.Content{}, err
		}
		return repositoryService.Get(ctx, normalizedPath)
	}

	value, err := orchestratorService.GetLocal(ctx, normalizedPath)
	if err == nil {
		return value, nil
	}
	if !faults.IsCategory(err, faults.NotFoundError) {
		return resourcedomain.Content{}, err
	}
	return orchestratorService.GetRemote(ctx, normalizedPath)
}

func applyCopyOverrideAttributes(value resourcedomain.Content, overrideAttributes string) (resourcedomain.Content, error) {
	if strings.TrimSpace(overrideAttributes) == "" {
		return value, nil
	}

	normalized, err := resourcedomain.Normalize(value.Value)
	if err != nil {
		return resourcedomain.Content{}, err
	}
	payloadMap, ok := normalized.(map[string]any)
	if !ok {
		return resourcedomain.Content{}, cliutil.ValidationError("--override-attributes requires an object payload", nil)
	}
	if err := cliutil.ApplyPointerAssignmentsObject(payloadMap, overrideAttributes); err != nil {
		return resourcedomain.Content{}, err
	}
	normalizedValue, err := resourcedomain.Normalize(payloadMap)
	if err != nil {
		return resourcedomain.Content{}, err
	}
	return resourcedomain.Content{
		Value:      normalizedValue,
		Descriptor: value.Descriptor,
	}, nil
}
