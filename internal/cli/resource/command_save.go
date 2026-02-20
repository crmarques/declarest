package resource

import (
	"sort"
	"strings"

	resourcesave "github.com/crmarques/declarest/internal/app/resource/save"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

const handleSecretsAllSentinel = "__all__"

func newSaveCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var input common.InputFlags
	var asItems bool
	var asOneResource bool
	var ignore bool
	var handleSecrets string
	var force bool

	command := &cobra.Command{
		Use:   "save [path]",
		Short: "Save resource value into repository",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			value, hasInput, err := decodeOptionalResourceInput(command, input)
			if err != nil {
				return err
			}

			handleSecretsEnabled, requestedSecretCandidates, err := parseSaveHandleSecretsFlag(command, handleSecrets)
			if err != nil {
				return err
			}

			orchestratorService, err := common.RequireOrchestrator(deps)
			if err != nil {
				return err
			}
			repositoryService, err := common.RequireResourceStore(deps)
			if err != nil {
				return err
			}

			return resourcesave.Execute(
				command.Context(),
				resourcesave.Dependencies{
					Orchestrator: orchestratorService,
					Repository:   repositoryService,
					Metadata:     deps.Metadata,
					Secrets:      deps.Secrets,
				},
				resolvedPath,
				value,
				hasInput,
				resourcesave.ExecuteOptions{
					AsItems:                   asItems,
					AsOneResource:             asOneResource,
					Ignore:                    ignore,
					Force:                     force,
					HandleSecretsEnabled:      handleSecretsEnabled,
					RequestedSecretCandidates: requestedSecretCandidates,
				},
			)
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	common.BindInputFlags(command, &input)
	command.Flags().BoolVar(&asItems, "as-items", false, "save list payload entries as individual resources")
	command.Flags().BoolVar(&asOneResource, "as-one-resource", false, "save payload as one resource file")
	command.Flags().BoolVar(&ignore, "ignore", false, "ignore plaintext-secret safety validation when saving")
	command.Flags().StringVar(&handleSecrets, "handle-secrets", "", "detect, store, and mask plaintext secrets while saving (optional comma-separated attributes)")
	command.Flags().BoolVar(&force, "force", false, "override existing repository resources")
	handleSecretsFlag := command.Flags().Lookup("handle-secrets")
	handleSecretsFlag.NoOptDefVal = handleSecretsAllSentinel
	return command
}

func parseSaveHandleSecretsFlag(command *cobra.Command, rawValue string) (bool, []string, error) {
	flag := command.Flags().Lookup("handle-secrets")
	if flag == nil || !flag.Changed {
		return false, nil, nil
	}

	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" || trimmed == handleSecretsAllSentinel {
		return true, nil, nil
	}

	items := strings.Split(trimmed, ",")
	requested := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, raw := range items {
		value := strings.TrimSpace(raw)
		if value == "" {
			return false, nil, common.ValidationError("--handle-secrets contains an empty attribute value", nil)
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		requested = append(requested, value)
	}
	sort.Strings(requested)

	return true, requested, nil
}
