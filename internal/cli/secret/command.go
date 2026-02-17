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
		newGetCommand(deps, globalFlags),
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

func newGetCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Read a secret",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := common.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := common.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			value, err := secretProvider.Get(command.Context(), args[0])
			if err != nil {
				return err
			}

			return common.WriteOutput(command, outputFormat, value, func(w io.Writer, item string) error {
				_, writeErr := io.WriteString(w, item+"\n")
				return writeErr
			})
		},
	}
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
