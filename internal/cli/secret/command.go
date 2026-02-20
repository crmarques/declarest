package secret

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func NewCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "secret",
		Short: "Manage secrets",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newInitCommand(deps),
		newStoreCommand(deps),
		newGetCommand(deps),
		newDeleteCommand(deps),
		newListCommand(deps, globalFlags),
		newMaskCommand(deps, globalFlags),
		newResolveCommand(deps, globalFlags),
		newNormalizeCommand(deps, globalFlags),
		newDetectCommand(deps, globalFlags),
	)

	return command
}

func newInitCommand(deps common.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize secret store",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			return secretProvider.Init(command.Context())
		},
	}
}

func newStoreCommand(deps common.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "store <key> <value>",
		Short: "Store a secret",
		Args:  cobra.ExactArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			return secretProvider.Store(command.Context(), args[0], args[1])
		},
	}
}

func newGetCommand(deps common.CommandDependencies) *cobra.Command {
	var pathFlag string
	var keyFlag string

	command := &cobra.Command{
		Use:   "get [path] [key]",
		Short: "Read one secret or all secrets for a path",
		Example: strings.Join([]string{
			"  declarest secret get /customers/acme",
			"  declarest secret get /customers/acme apiToken",
			"  declarest secret get --path /customers/acme",
			"  declarest secret get --path /customers/acme --key apiToken",
			"  declarest secret get /customers/acme:apiToken",
		}, "\n"),
		Args: cobra.MaximumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			request, err := resolveSecretGetRequest(pathFlag, keyFlag, args)
			if err != nil {
				return err
			}

			if request.ListByPath {
				return writeSecretsByPath(command.Context(), command.OutOrStdout(), secretProvider, request.Path)
			}

			value, err := secretProvider.Get(command.Context(), request.ResolvedKey())
			if err != nil {
				return err
			}
			_, err = io.WriteString(command.OutOrStdout(), value+"\n")
			return err
		},
	}

	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	command.Flags().StringVar(&keyFlag, "key", "", "secret key under --path")
	return command
}

type secretGetRequest struct {
	Path       string
	Key        string
	DirectKey  string
	ListByPath bool
}

func (r secretGetRequest) ResolvedKey() string {
	if strings.TrimSpace(r.DirectKey) != "" {
		return r.DirectKey
	}
	if strings.TrimSpace(r.Path) == "" {
		return ""
	}
	return strings.TrimSpace(r.Path) + ":" + strings.TrimSpace(r.Key)
}

func resolveSecretGetRequest(pathFlag string, keyFlag string, args []string) (secretGetRequest, error) {
	normalizedPathFlag, hasPathFlag, err := normalizeGetPathFlag(pathFlag)
	if err != nil {
		return secretGetRequest{}, err
	}
	normalizedKeyFlag := strings.TrimSpace(keyFlag)

	switch len(args) {
	case 0:
		return resolveSecretGetFromFlagsOnly(normalizedPathFlag, hasPathFlag, normalizedKeyFlag)
	case 1:
		return resolveSecretGetFromSingleArg(normalizedPathFlag, hasPathFlag, normalizedKeyFlag, args[0])
	case 2:
		return resolveSecretGetFromPathAndKeyArgs(normalizedPathFlag, hasPathFlag, normalizedKeyFlag, args[0], args[1])
	default:
		return secretGetRequest{}, common.ValidationError("secret get accepts at most two positional arguments", nil)
	}
}

func resolveSecretGetFromFlagsOnly(pathFlag string, hasPathFlag bool, keyFlag string) (secretGetRequest, error) {
	if !hasPathFlag {
		if keyFlag != "" {
			return secretGetRequest{}, common.ValidationError("--key requires --path", nil)
		}
		return secretGetRequest{}, common.ValidationError("secret get requires a key, path, or --path", nil)
	}

	if keyFlag == "" {
		return secretGetRequest{Path: pathFlag, ListByPath: true}, nil
	}
	return secretGetRequest{Path: pathFlag, Key: keyFlag}, nil
}

func resolveSecretGetFromSingleArg(pathFlag string, hasPathFlag bool, keyFlag string, rawArg string) (secretGetRequest, error) {
	arg := strings.TrimSpace(rawArg)
	if arg == "" {
		return secretGetRequest{}, common.ValidationError("secret get argument must not be empty", nil)
	}

	if hasPathFlag {
		if keyFlag != "" && keyFlag != arg {
			return secretGetRequest{}, common.ValidationError("flag --key conflicts with positional key argument", nil)
		}
		if keyFlag != "" {
			return secretGetRequest{Path: pathFlag, Key: keyFlag}, nil
		}
		return secretGetRequest{Path: pathFlag, Key: arg}, nil
	}

	if keyFlag != "" {
		normalizedPathArg, err := normalizeSecretPathForGet(arg)
		if err != nil {
			return secretGetRequest{}, err
		}
		return secretGetRequest{Path: normalizedPathArg, Key: keyFlag}, nil
	}

	if strings.HasPrefix(arg, "/") && strings.Contains(arg, ":") {
		pathFromComposite, keyFromComposite, composite := splitSecretPathKeyArg(arg)
		if !composite {
			return secretGetRequest{}, common.ValidationError("invalid secret target format: expected <path>:<key>", nil)
		}
		return secretGetRequest{Path: pathFromComposite, Key: keyFromComposite}, nil
	}

	if strings.HasPrefix(arg, "/") {
		normalizedPathArg, err := normalizeSecretPathForGet(arg)
		if err != nil {
			return secretGetRequest{}, err
		}
		return secretGetRequest{Path: normalizedPathArg, ListByPath: true}, nil
	}

	// Backward-compatible direct key mode.
	return secretGetRequest{DirectKey: arg}, nil
}

func resolveSecretGetFromPathAndKeyArgs(
	pathFlag string,
	hasPathFlag bool,
	keyFlag string,
	rawPathArg string,
	rawKeyArg string,
) (secretGetRequest, error) {
	normalizedPathArg, err := normalizeSecretPathForGet(rawPathArg)
	if err != nil {
		return secretGetRequest{}, err
	}

	keyArg := strings.TrimSpace(rawKeyArg)
	if keyArg == "" {
		return secretGetRequest{}, common.ValidationError("secret key must not be empty", nil)
	}

	if hasPathFlag && pathFlag != normalizedPathArg {
		return secretGetRequest{}, common.ValidationError("flag --path conflicts with positional path argument", nil)
	}
	if keyFlag != "" && keyFlag != keyArg {
		return secretGetRequest{}, common.ValidationError("flag --key conflicts with positional key argument", nil)
	}

	return secretGetRequest{Path: normalizedPathArg, Key: keyArg}, nil
}

func normalizeGetPathFlag(pathFlag string) (string, bool, error) {
	trimmed := strings.TrimSpace(pathFlag)
	if trimmed == "" {
		return "", false, nil
	}
	normalized, err := normalizeSecretPathForGet(trimmed)
	if err != nil {
		return "", true, err
	}
	return normalized, true, nil
}

func normalizeSecretPathForGet(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", common.ValidationError("path is required", nil)
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "", common.ValidationError("path must be absolute", nil)
	}
	return resource.NormalizeLogicalPath(trimmed)
}

func splitSecretPathKeyArg(value string) (string, string, bool) {
	if !strings.HasPrefix(value, "/") {
		return "", "", false
	}

	index := strings.Index(value, ":")
	if index <= 0 {
		return "", "", false
	}

	pathPart := strings.TrimSpace(value[:index])
	keyPart := strings.TrimSpace(value[index+1:])
	if keyPart == "" {
		return "", "", false
	}

	normalizedPath, err := normalizeSecretPathForGet(pathPart)
	if err != nil {
		return "", "", false
	}

	return normalizedPath, keyPart, true
}

func writeSecretsByPath(
	ctx context.Context,
	writer io.Writer,
	secretProvider secretdomain.SecretProvider,
	logicalPath string,
) error {
	keys, err := secretProvider.List(ctx)
	if err != nil {
		return err
	}

	prefix := logicalPath + ":"
	matchingKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		if strings.HasPrefix(strings.TrimSpace(key), prefix) {
			matchingKeys = append(matchingKeys, key)
		}
	}
	sort.Strings(matchingKeys)

	if len(matchingKeys) == 0 {
		return faults.NewTypedError(faults.NotFoundError, "secret path not found", nil)
	}

	lines := make([]string, 0, len(matchingKeys))
	for _, fullKey := range matchingKeys {
		value, err := secretProvider.Get(ctx, fullKey)
		if err != nil {
			return err
		}

		displayKey := strings.TrimPrefix(fullKey, prefix)
		if strings.TrimSpace(displayKey) == "" {
			displayKey = fullKey
		}
		lines = append(lines, displayKey+"="+value)
	}

	_, err = io.WriteString(writer, strings.Join(lines, "\n")+"\n")
	return err
}

func newDeleteCommand(deps common.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <key>",
		Short: "Delete a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			return secretProvider.Delete(command.Context(), args[0])
		},
	}
}

func newListCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List secrets",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			items, err := secretProvider.List(command.Context())
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, items, nil)
		},
	}
}

func newMaskCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "mask",
		Short: "Mask secret values in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			masked, err := secretProvider.MaskPayload(command.Context(), value)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, masked, nil)
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newResolveCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve secret placeholders in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			resolved, err := secretProvider.ResolvePayload(command.Context(), value)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, resolved, nil)
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newNormalizeCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "normalize",
		Short: "Normalize secret placeholders",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := common.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			normalized, err := secretProvider.NormalizeSecretPlaceholders(command.Context(), value)
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, normalized, nil)
		},
	}

	common.BindInputFlags(command, &input)
	return command
}

func newDetectCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var input common.InputFlags
	var pathFlag string
	var fix bool
	var secretAttribute string

	command := &cobra.Command{
		Use:   "detect [path]",
		Short: "Detect potential secrets in payload or local resources",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := common.ResolvePathInput(pathFlag, args, false)
			if err != nil {
				return err
			}

			value, hasInput, err := decodeDetectInput(command, input)
			if err != nil {
				return err
			}

			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			if hasInput {
				keys, detectErr := secretProvider.DetectSecretCandidates(command.Context(), value)
				if detectErr != nil {
					return detectErr
				}

				appliedKeys, filterErr := resolveDetectSecretAttributes(keys, secretAttribute)
				if filterErr != nil {
					return filterErr
				}

				if fix {
					if strings.TrimSpace(resolvedPath) == "" {
						return common.ValidationError("path is required", nil)
					}
					if err := applyDetectedSecretAttributes(command.Context(), deps, resolvedPath, appliedKeys); err != nil {
						return err
					}
				} else if strings.TrimSpace(resolvedPath) != "" {
					return common.ValidationError("path input requires --fix when detecting from input payload", nil)
				}

				return common.WriteOutput(command, outputFormat, appliedKeys, nil)
			}

			scanPath := strings.TrimSpace(resolvedPath)
			if scanPath == "" {
				scanPath = "/"
			}

			results, err := detectSecretCandidatesFromRepository(
				command.Context(),
				deps,
				secretProvider,
				scanPath,
				secretAttribute,
			)
			if err != nil {
				return err
			}

			if fix {
				for _, result := range results {
					if err := applyDetectedSecretAttributes(command.Context(), deps, result.LogicalPath, result.Attributes); err != nil {
						return err
					}
				}
			}

			return common.WriteOutput(command, outputFormat, results, nil)
		},
	}

	common.BindInputFlags(command, &input)
	common.BindPathFlag(command, &pathFlag)
	common.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = common.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVar(&fix, "fix", false, "write detected secret attributes to metadata")
	command.Flags().StringVar(&secretAttribute, "secret-attribute", "", "apply only one detected secret attribute")
	return command
}

type detectedResourceSecrets struct {
	LogicalPath string
	Attributes  []string
}

func decodeDetectInput(command *cobra.Command, flags common.InputFlags) (resource.Value, bool, error) {
	data, err := common.ReadInput(command, flags)
	if err != nil {
		if isInputRequiredError(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var value resource.Value
	switch flags.Format {
	case "", common.OutputJSON:
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, false, common.ValidationError("invalid json input", err)
		}
	case common.OutputYAML:
		if err := yaml.Unmarshal(data, &value); err != nil {
			return nil, false, common.ValidationError("invalid yaml input", err)
		}
	default:
		return nil, false, common.ValidationError("invalid input format: use json or yaml", nil)
	}

	return value, true, nil
}

func detectSecretCandidatesFromRepository(
	ctx context.Context,
	deps common.CommandDependencies,
	secretProvider secretdomain.SecretProvider,
	scanPath string,
	secretAttribute string,
) ([]detectedResourceSecrets, error) {
	orchestratorService, err := common.RequireOrchestrator(deps)
	if err != nil {
		return nil, err
	}

	items, err := orchestratorService.ListLocal(ctx, scanPath, orchestratordomain.ListPolicy{Recursive: true})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i int, j int) bool {
		return items[i].LogicalPath < items[j].LogicalPath
	})

	results := make([]detectedResourceSecrets, 0, len(items))
	requestedAttribute := strings.TrimSpace(secretAttribute)
	requestedAttributeMatched := false

	for _, item := range items {
		if strings.TrimSpace(item.LogicalPath) == "" {
			continue
		}

		value, err := orchestratorService.GetLocal(ctx, item.LogicalPath)
		if err != nil {
			return nil, err
		}

		keys, err := secretProvider.DetectSecretCandidates(ctx, value)
		if err != nil {
			return nil, err
		}

		filtered, matched := filterDetectedSecretAttributes(keys, requestedAttribute)
		if !matched {
			continue
		}
		if requestedAttribute != "" {
			requestedAttributeMatched = true
		}

		results = append(results, detectedResourceSecrets{
			LogicalPath: item.LogicalPath,
			Attributes:  filtered,
		})
	}

	if requestedAttribute != "" && !requestedAttributeMatched {
		return nil, common.ValidationError(
			"requested --secret-attribute was not detected",
			nil,
		)
	}

	return results, nil
}

func filterDetectedSecretAttributes(keys []string, secretAttribute string) ([]string, bool) {
	normalizedKeys := dedupeAndSortStrings(keys)
	if len(normalizedKeys) == 0 {
		return nil, false
	}

	attribute := strings.TrimSpace(secretAttribute)
	if attribute == "" {
		return normalizedKeys, true
	}

	for _, key := range normalizedKeys {
		if key == attribute {
			return []string{attribute}, true
		}
	}
	return nil, false
}

func isInputRequiredError(err error) bool {
	if !isTypedErrorCategory(err, faults.ValidationError) {
		return false
	}
	return strings.Contains(err.Error(), "input is required")
}

func resolveDetectSecretAttributes(keys []string, secretAttribute string) ([]string, error) {
	filtered, matched := filterDetectedSecretAttributes(keys, secretAttribute)
	if matched {
		return filtered, nil
	}

	if strings.TrimSpace(secretAttribute) != "" {
		return nil, common.ValidationError(
			"requested --secret-attribute was not detected",
			nil,
		)
	}

	return []string{}, nil
}

func applyDetectedSecretAttributes(
	ctx context.Context,
	deps common.CommandDependencies,
	logicalPath string,
	detected []string,
) error {
	metadataService, err := common.RequireMetadataService(deps)
	if err != nil {
		return err
	}

	if len(detected) == 0 {
		return nil
	}

	currentMetadata, err := metadataService.Get(ctx, logicalPath)
	if err != nil {
		if !isTypedErrorCategory(err, faults.NotFoundError) {
			return err
		}
		currentMetadata = metadatadomain.ResourceMetadata{}
	}

	currentMetadata.SecretsFromAttributes = mergeSecretAttributes(
		currentMetadata.SecretsFromAttributes,
		detected,
	)

	return metadataService.Set(ctx, logicalPath, currentMetadata)
}

func mergeSecretAttributes(existing []string, detected []string) []string {
	merged := make([]string, 0, len(existing)+len(detected))
	seen := make(map[string]struct{}, len(existing)+len(detected))

	for _, raw := range existing {
		attribute := strings.TrimSpace(raw)
		if attribute == "" {
			continue
		}
		if _, found := seen[attribute]; found {
			continue
		}
		seen[attribute] = struct{}{}
		merged = append(merged, attribute)
	}
	for _, raw := range detected {
		attribute := strings.TrimSpace(raw)
		if attribute == "" {
			continue
		}
		if _, found := seen[attribute]; found {
			continue
		}
		seen[attribute] = struct{}{}
		merged = append(merged, attribute)
	}

	sort.Strings(merged)
	return merged
}

func dedupeAndSortStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	items := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if _, found := seen[value]; found {
			continue
		}
		seen[value] = struct{}{}
		items = append(items, value)
	}
	sort.Strings(items)
	return items
}

func isTypedErrorCategory(err error, category faults.ErrorCategory) bool {
	if err == nil {
		return false
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		return false
	}
	return typedErr.Category == category
}
