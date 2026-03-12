package resource

import (
	"fmt"
	"sort"
	"strings"

	resourcesave "github.com/crmarques/declarest/internal/app/resource/save"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	resourceinputapp "github.com/crmarques/declarest/internal/cli/resource/input"
	"github.com/spf13/cobra"
)

const secretAttributesAllSentinel = "__all__"

const (
	saveModeAuto   = "auto"
	saveModeItems  = "items"
	saveModeSingle = "single"
)

func newSaveCommand(deps cliutil.CommandDependencies) *cobra.Command {
	var pathFlag string
	var input cliutil.InputFlags
	var excludeItemsFlag []string
	var mode string
	var secret bool
	var allowPlaintext bool
	var secretAttributes string
	var force bool
	var push bool
	var commitMessage string

	command := &cobra.Command{
		Use:   "save [path]",
		Short: "Save resource value into repository",
		Example: strings.Join([]string{
			"  declarest resource save /customers/acme",
			"  declarest resource save /customers/acme --payload payload.json",
			"  declarest resource save /admin/realms --exclude master --exclude realm1",
			"  cat payload.json | declarest resource save /customers/acme --payload -",
			"  declarest resource save /customers/ --mode items < customers.json",
			"  declarest resource save /customers/acme --secret-attributes",
			"  declarest resource save /projects/platform/secrets/private-key --payload private.key --secret",
			"  declarest resource save /customers/acme --force",
			"  declarest --context git resource save /customers/acme --payload payload.json --force --push",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, true)
			if err != nil {
				return err
			}

			value, hasInput, err := resourceinputapp.DecodeOptionalMutationPayloadInput(command, input)
			if err != nil {
				return err
			}

			secretAttributesEnabled, requestedSecretAttributes, err := parseSaveSecretAttributesFlag(command, secretAttributes)
			if err != nil {
				return err
			}
			asItems, asOneResource, err := parseSaveMode(mode)
			if err != nil {
				return err
			}
			excludeItems, err := parseExcludeFlag(command, excludeItemsFlag)
			if err != nil {
				return err
			}

			cfg, err := resolveActiveResourceContext(command.Context(), deps, nil)
			if err != nil {
				return err
			}
			if err := validateRepositoryPushFlag(cfg, push); err != nil {
				return err
			}
			if err := ensureCleanGitWorktreeForAutoCommit(command.Context(), deps, cfg, "resource save"); err != nil {
				return err
			}
			commitMessage, err := resolveRepositoryCommitMessage(
				command,
				fmt.Sprintf("declarest: save resource %s", resolvedPath),
				commitMessage,
			)
			if err != nil {
				return err
			}

			if err := resourcesave.Execute(
				command.Context(),
				deps,
				resolvedPath,
				value,
				hasInput,
				resourcesave.ExecuteOptions{
					AsItems:                   asItems,
					AsOneResource:             asOneResource,
					Secret:                    secret,
					AllowPlaintext:            allowPlaintext,
					Force:                     force,
					SecretAttributesEnabled:   secretAttributesEnabled,
					RequestedSecretAttributes: requestedSecretAttributes,
					SkipItems:                 excludeItems,
				},
			); err != nil {
				return err
			}

			return commitAndMaybePushRepository(command.Context(), deps, cfg, commitMessage, push)
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	cliutil.BindResourceInputFlags(command, &input)
	bindExcludeFlag(command, &excludeItemsFlag)
	if flag := command.Flags().Lookup("payload"); flag != nil {
		flag.Usage = "payload file path (use '-' to read object from stdin); also accepts inline JSON/YAML or JSON Pointer assignments (/a=b,/c/d=e); binary requires file or stdin"
	}
	command.Flags().StringVar(&mode, "mode", saveModeAuto, "save mode: auto, items, or single")
	cliutil.RegisterFlagValueCompletions(command, "mode", []string{saveModeAuto, saveModeItems, saveModeSingle})
	command.Flags().BoolVar(&secret, "secret", false, "store the whole resource payload in the secret store and persist only a placeholder")
	command.Flags().BoolVar(&allowPlaintext, "allow-plaintext", false, "acknowledge saving resources that may contain plaintext secrets")
	command.Flags().StringVar(&secretAttributes, "secret-attributes", "", "detect, store, and mask individual secret attributes (optional comma-separated JSON pointers; structured payloads only)")
	command.Flags().BoolVar(&force, "force", false, "override existing repository resources")
	command.Flags().BoolVar(&push, "push", false, "push git repository changes after save (git repositories with remote only)")
	bindRepositoryCommitMessageFlags(command, &commitMessage)
	secretAttributesFlag := command.Flags().Lookup("secret-attributes")
	secretAttributesFlag.NoOptDefVal = secretAttributesAllSentinel
	return command
}

func parseSaveMode(rawValue string) (bool, bool, error) {
	switch strings.TrimSpace(rawValue) {
	case "", saveModeAuto:
		return false, false, nil
	case saveModeItems:
		return true, false, nil
	case saveModeSingle:
		return false, true, nil
	default:
		return false, false, cliutil.ValidationError("flag --mode must be one of: auto, items, single", nil)
	}
}

func parseSaveSecretAttributesFlag(command *cobra.Command, rawValue string) (bool, []string, error) {
	flag := command.Flags().Lookup("secret-attributes")
	if flag == nil || !flag.Changed {
		return false, nil, nil
	}

	trimmed := strings.TrimSpace(rawValue)
	if trimmed == "" || trimmed == secretAttributesAllSentinel {
		return true, nil, nil
	}

	items := strings.Split(trimmed, ",")
	requested := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, raw := range items {
		value := strings.TrimSpace(raw)
		if value == "" {
			return false, nil, cliutil.ValidationError("--secret-attributes contains an empty attribute value", nil)
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
