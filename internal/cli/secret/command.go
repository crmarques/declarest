package secret

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/crmarques/declarest/faults"
	detectapp "github.com/crmarques/declarest/internal/app/secret/detect"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	"github.com/crmarques/declarest/resource"
	secretdomain "github.com/crmarques/declarest/secrets"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

func NewCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "secret",
		Short: "Manage secrets",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newInitCommand(deps),
		newSetCommand(deps),
		newGetCommand(deps),
		newListCommand(deps, globalFlags),
		newDeleteCommand(deps),
		newMaskCommand(deps, globalFlags),
		newResolveCommand(deps, globalFlags),
		newNormalizeCommand(deps, globalFlags),
		newDetectCommand(deps, globalFlags),
	)

	return command
}

func newInitCommand(deps cliutil.CommandDependencies) *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize secret store",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			return secretProvider.Init(command.Context())
		},
	}
}

func newSetCommand(deps cliutil.CommandDependencies) *cobra.Command {
	var pathFlag string
	var keyFlag string

	command := &cobra.Command{
		Use:     "set [path] [key] [value]",
		Aliases: []string{"store"},
		Short:   "Set a secret",
		Example: strings.Join([]string{
			"  declarest secret set apiToken super-secret",
			"  declarest secret set /customers/acme /apiToken super-secret",
			"  declarest secret set --path /customers/acme --key /apiToken super-secret",
			"  declarest secret set /customers/acme:/apiToken super-secret",
		}, "\n"),
		Args: cobra.RangeArgs(1, 3),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			request, err := resolveSecretSetRequest(pathFlag, keyFlag, args)
			if err != nil {
				return err
			}

			return secretProvider.Store(command.Context(), request.Target.ResolvedKey(), request.Value)
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().StringVar(&keyFlag, "key", "", "secret key under --path")
	return command
}

func newGetCommand(deps cliutil.CommandDependencies) *cobra.Command {
	var pathFlag string
	var keyFlag string

	command := &cobra.Command{
		Use:   "get [path] [key]",
		Short: "Read one secret",
		Example: strings.Join([]string{
			"  declarest secret get apiToken",
			"  declarest secret list /customers/acme",
			"  declarest secret get /customers/acme /apiToken",
			"  declarest secret get --path /customers/acme --key /apiToken",
			"  declarest secret get /customers/acme:/apiToken",
		}, "\n"),
		Args: cobra.RangeArgs(0, 2),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			target, err := resolveSecretTargetRequest("get", pathFlag, keyFlag, args)
			if err != nil {
				return err
			}

			value, err := secretProvider.Get(command.Context(), target.ResolvedKey())
			if err != nil {
				return err
			}
			_, err = io.WriteString(command.OutOrStdout(), value+"\n")
			return err
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().StringVar(&keyFlag, "key", "", "secret key under --path")
	return command
}

func newListCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var pathFlag string
	var recursive bool

	command := &cobra.Command{
		Use:   "list [path]",
		Short: "List stored secret keys",
		Example: strings.Join([]string{
			"  declarest secret list",
			"  declarest secret list /customers/acme",
			"  declarest secret list --path /customers/acme",
			"  declarest secret list /projects/test --recursive",
			"  declarest secret list /customers/acme --output json",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			request, err := resolveSecretListRequest(pathFlag, recursive, args)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}
			if globalFlags != nil && globalFlags.Output == cliutil.OutputAuto {
				outputFormat = cliutil.OutputAuto
			}

			items, err := listSecretKeys(command.Context(), secretProvider, request)
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(command, outputFormat, items, func(w io.Writer, items []string) error {
				for _, item := range items {
					if _, writeErr := fmt.Fprintln(w, item); writeErr != nil {
						return writeErr
					}
				}
				return nil
			})
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVarP(&recursive, "recursive", "r", false, "include descendant secret paths")
	return command
}

func newDeleteCommand(deps cliutil.CommandDependencies) *cobra.Command {
	var pathFlag string
	var keyFlag string

	command := &cobra.Command{
		Use:   "delete [path] [key]",
		Short: "Delete one secret",
		Example: strings.Join([]string{
			"  declarest secret delete apiToken",
			"  declarest secret delete /customers/acme /apiToken",
			"  declarest secret delete --path /customers/acme --key /apiToken",
			"  declarest secret delete /customers/acme:/apiToken",
		}, "\n"),
		Args: cobra.RangeArgs(0, 2),
		RunE: func(command *cobra.Command, args []string) error {
			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			target, err := resolveSecretTargetRequest("delete", pathFlag, keyFlag, args)
			if err != nil {
				return err
			}

			return secretProvider.Delete(command.Context(), target.ResolvedKey())
		},
	}

	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().StringVar(&keyFlag, "key", "", "secret key under --path")
	return command
}

type secretTarget struct {
	Path string
	Key  string
}

func (r secretTarget) ResolvedKey() string {
	if strings.TrimSpace(r.Path) == "" {
		return strings.TrimSpace(r.Key)
	}
	return strings.TrimSpace(r.Path) + ":" + strings.TrimSpace(r.Key)
}

type secretSetRequest struct {
	Target secretTarget
	Value  string
}

type secretListRequest struct {
	Path      string
	HasPath   bool
	Recursive bool
}

func resolveSecretTargetRequest(action string, pathFlag string, keyFlag string, args []string) (secretTarget, error) {
	normalizedPathFlag, hasPathFlag, err := normalizeSecretPathFlag(pathFlag)
	if err != nil {
		return secretTarget{}, err
	}
	normalizedKeyFlag := strings.TrimSpace(keyFlag)
	if normalizedKeyFlag != "" && !hasPathFlag {
		return secretTarget{}, cliutil.ValidationError("--key requires --path", nil)
	}

	switch len(args) {
	case 0:
		if !hasPathFlag {
			return secretTarget{}, cliutil.ValidationError(
				fmt.Sprintf("secret %s requires a key; use 'declarest secret list <path>' to inspect path-scoped keys", action),
				nil,
			)
		}
		if normalizedKeyFlag == "" {
			return secretTarget{}, cliutil.ValidationError(
				fmt.Sprintf("secret %s requires a key; use 'declarest secret list %s' to inspect available keys", action, normalizedPathFlag),
				nil,
			)
		}
		return secretTarget{Path: normalizedPathFlag, Key: normalizedKeyFlag}, nil
	case 1:
		return resolveSecretTargetFromSingleArg(action, normalizedPathFlag, hasPathFlag, normalizedKeyFlag, args[0])
	case 2:
		return resolveSecretTargetFromPathAndKeyArgs(action, normalizedPathFlag, hasPathFlag, normalizedKeyFlag, args[0], args[1])
	default:
		return secretTarget{}, cliutil.ValidationError(
			fmt.Sprintf("secret %s accepts at most two positional arguments", action),
			nil,
		)
	}
}

func resolveSecretTargetFromSingleArg(
	action string,
	pathFlag string,
	hasPathFlag bool,
	keyFlag string,
	rawArg string,
) (secretTarget, error) {
	arg := strings.TrimSpace(rawArg)
	if arg == "" {
		return secretTarget{}, cliutil.ValidationError(fmt.Sprintf("secret %s argument must not be empty", action), nil)
	}

	if hasPathFlag {
		if keyFlag != "" && keyFlag != arg {
			return secretTarget{}, cliutil.ValidationError("flag --key conflicts with positional key argument", nil)
		}
		if keyFlag != "" {
			return secretTarget{Path: pathFlag, Key: keyFlag}, nil
		}
		return secretTarget{Path: pathFlag, Key: arg}, nil
	}

	if keyFlag != "" {
		return secretTarget{}, cliutil.ValidationError("--key requires --path", nil)
	}

	if strings.HasPrefix(arg, "/") {
		pathFromComposite, keyFromComposite, composite := splitSecretPathKeyArg(arg)
		if composite {
			return secretTarget{Path: pathFromComposite, Key: keyFromComposite}, nil
		}

		normalizedPathArg, err := normalizeSecretPathForInput(arg)
		if err != nil {
			return secretTarget{}, err
		}
		return secretTarget{}, cliutil.ValidationError(
			fmt.Sprintf("secret %s requires a key; use 'declarest secret list %s' to inspect available keys", action, normalizedPathArg),
			nil,
		)
	}

	return secretTarget{Key: arg}, nil
}

func resolveSecretTargetFromPathAndKeyArgs(
	_ string,
	pathFlag string,
	hasPathFlag bool,
	keyFlag string,
	rawPathArg string,
	rawKeyArg string,
) (secretTarget, error) {
	normalizedPathArg, err := normalizeSecretPathForInput(rawPathArg)
	if err != nil {
		return secretTarget{}, err
	}

	keyArg := strings.TrimSpace(rawKeyArg)
	if keyArg == "" {
		return secretTarget{}, cliutil.ValidationError("secret key must not be empty", nil)
	}

	if hasPathFlag && pathFlag != normalizedPathArg {
		return secretTarget{}, cliutil.ValidationError("flag --path conflicts with positional path argument", nil)
	}
	if keyFlag != "" && keyFlag != keyArg {
		return secretTarget{}, cliutil.ValidationError("flag --key conflicts with positional key argument", nil)
	}

	if hasPathFlag {
		normalizedPathArg = pathFlag
	}
	if keyFlag != "" {
		keyArg = keyFlag
	}

	return secretTarget{Path: normalizedPathArg, Key: keyArg}, nil
}

func resolveSecretSetRequest(pathFlag string, keyFlag string, args []string) (secretSetRequest, error) {
	normalizedPathFlag, hasPathFlag, err := normalizeSecretPathFlag(pathFlag)
	if err != nil {
		return secretSetRequest{}, err
	}
	normalizedKeyFlag := strings.TrimSpace(keyFlag)
	if normalizedKeyFlag != "" && !hasPathFlag {
		return secretSetRequest{}, cliutil.ValidationError("--key requires --path", nil)
	}

	switch len(args) {
	case 1:
		if !hasPathFlag || normalizedKeyFlag == "" {
			return secretSetRequest{}, cliutil.ValidationError(
				"secret set requires <key> <value> or <path> <key> <value>",
				nil,
			)
		}
		return secretSetRequest{
			Target: secretTarget{Path: normalizedPathFlag, Key: normalizedKeyFlag},
			Value:  args[0],
		}, nil
	case 2:
		return resolveSecretSetFromTwoArgs(normalizedPathFlag, hasPathFlag, normalizedKeyFlag, args[0], args[1])
	case 3:
		normalizedPathArg, err := normalizeSecretPathForInput(args[0])
		if err != nil {
			return secretSetRequest{}, err
		}
		keyArg := strings.TrimSpace(args[1])
		if keyArg == "" {
			return secretSetRequest{}, cliutil.ValidationError("secret key must not be empty", nil)
		}

		if hasPathFlag && normalizedPathFlag != normalizedPathArg {
			return secretSetRequest{}, cliutil.ValidationError("flag --path conflicts with positional path argument", nil)
		}
		if normalizedKeyFlag != "" && normalizedKeyFlag != keyArg {
			return secretSetRequest{}, cliutil.ValidationError("flag --key conflicts with positional key argument", nil)
		}

		if hasPathFlag {
			normalizedPathArg = normalizedPathFlag
		}
		if normalizedKeyFlag != "" {
			keyArg = normalizedKeyFlag
		}

		return secretSetRequest{
			Target: secretTarget{Path: normalizedPathArg, Key: keyArg},
			Value:  args[2],
		}, nil
	default:
		return secretSetRequest{}, cliutil.ValidationError("secret set accepts at most three positional arguments", nil)
	}
}

func resolveSecretSetFromTwoArgs(
	pathFlag string,
	hasPathFlag bool,
	keyFlag string,
	rawTargetArg string,
	valueArg string,
) (secretSetRequest, error) {
	targetArg := strings.TrimSpace(rawTargetArg)
	if targetArg == "" {
		return secretSetRequest{}, cliutil.ValidationError("secret key must not be empty", nil)
	}

	if hasPathFlag {
		if keyFlag != "" && keyFlag != targetArg {
			return secretSetRequest{}, cliutil.ValidationError("flag --key conflicts with positional key argument", nil)
		}
		if keyFlag == "" {
			keyFlag = targetArg
		}

		return secretSetRequest{
			Target: secretTarget{Path: pathFlag, Key: keyFlag},
			Value:  valueArg,
		}, nil
	}

	if strings.HasPrefix(targetArg, "/") {
		pathFromComposite, keyFromComposite, composite := splitSecretPathKeyArg(targetArg)
		if composite {
			return secretSetRequest{
				Target: secretTarget{Path: pathFromComposite, Key: keyFromComposite},
				Value:  valueArg,
			}, nil
		}

		if _, err := normalizeSecretPathForInput(targetArg); err != nil {
			return secretSetRequest{}, err
		}
		return secretSetRequest{}, cliutil.ValidationError(
			"secret set requires a key; use 'declarest secret set <path> <key> <value>'",
			nil,
		)
	}

	return secretSetRequest{
		Target: secretTarget{Key: targetArg},
		Value:  valueArg,
	}, nil
}

func resolveSecretListRequest(pathFlag string, recursive bool, args []string) (secretListRequest, error) {
	normalizedPathFlag, hasPathFlag, err := normalizeSecretPathFlag(pathFlag)
	if err != nil {
		return secretListRequest{}, err
	}

	switch len(args) {
	case 0:
		return secretListRequest{Path: normalizedPathFlag, HasPath: hasPathFlag, Recursive: recursive}, nil
	case 1:
		arg := strings.TrimSpace(args[0])
		if arg == "" {
			return secretListRequest{}, cliutil.ValidationError("secret list path must not be empty", nil)
		}

		if strings.HasPrefix(arg, "/") {
			if _, _, composite := splitSecretPathKeyArg(arg); composite {
				return secretListRequest{}, cliutil.ValidationError(
					"secret list accepts only a path; use 'declarest secret get <path>:<key>' to read a value",
					nil,
				)
			}
		}

		normalizedPathArg, err := normalizeSecretPathForInput(arg)
		if err != nil {
			return secretListRequest{}, err
		}
		if hasPathFlag && normalizedPathFlag != normalizedPathArg {
			return secretListRequest{}, cliutil.ValidationError("flag --path conflicts with positional path argument", nil)
		}
		return secretListRequest{Path: normalizedPathArg, HasPath: true, Recursive: recursive}, nil
	default:
		return secretListRequest{}, cliutil.ValidationError("secret list accepts at most one positional path argument", nil)
	}
}

func normalizeSecretPathFlag(pathFlag string) (string, bool, error) {
	trimmed := strings.TrimSpace(pathFlag)
	if trimmed == "" {
		return "", false, nil
	}
	normalized, err := normalizeSecretPathForInput(trimmed)
	if err != nil {
		return "", true, err
	}
	return normalized, true, nil
}

func normalizeSecretPathForInput(rawPath string) (string, error) {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", cliutil.ValidationError("path is required", nil)
	}
	if !strings.HasPrefix(trimmed, "/") {
		return "", cliutil.ValidationError("path must be absolute", nil)
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

	normalizedPath, err := normalizeSecretPathForInput(pathPart)
	if err != nil {
		return "", "", false
	}

	return normalizedPath, keyPart, true
}

func listSecretKeys(
	ctx context.Context,
	secretProvider secretdomain.SecretProvider,
	request secretListRequest,
) ([]string, error) {
	keys, err := secretProvider.List(ctx)
	if err != nil {
		return nil, err
	}

	if !request.HasPath {
		return renderAllSecretKeys(keys), nil
	}

	normalizedPath := normalizeSecretStoreLookupKey(request.Path)
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		normalizedKey := normalizeSecretStoreLookupKey(key)
		pathPart, keyPart, composite := splitStoredSecretPathKey(normalizedKey)
		if !composite {
			continue
		}

		displayKey, matches := renderPathScopedSecretListKey(normalizedPath, pathPart, keyPart, request.Recursive || request.Path == "/")
		if !matches {
			continue
		}
		items = append(items, displayKey)
	}

	sort.Strings(items)
	if len(items) == 0 {
		return nil, faults.NewTypedError(faults.NotFoundError, "secret path not found", nil)
	}
	return items, nil
}

func renderPathScopedSecretListKey(
	requestPath string,
	pathPart string,
	keyPart string,
	recursive bool,
) (string, bool) {
	normalizedRequestPath := strings.Trim(requestPath, "/")
	normalizedPathPart := strings.Trim(pathPart, "/")
	if normalizedPathPart == "" || strings.TrimSpace(keyPart) == "" {
		return "", false
	}

	if normalizedRequestPath == "" {
		return "/" + normalizedPathPart + ":" + keyPart, true
	}

	if normalizedPathPart == normalizedRequestPath {
		return keyPart, true
	}

	if !recursive {
		return "", false
	}

	prefix := normalizedRequestPath + "/"
	if !strings.HasPrefix(normalizedPathPart, prefix) {
		return "", false
	}

	relativePath := strings.TrimPrefix(normalizedPathPart, prefix)
	if relativePath == "" {
		return "", false
	}

	return "/" + relativePath + ":" + keyPart, true
}

func renderAllSecretKeys(keys []string) []string {
	items := make([]string, 0, len(keys))
	for _, key := range keys {
		displayKey := displaySecretKey(key)
		if displayKey == "" {
			continue
		}
		items = append(items, displayKey)
	}
	sort.Strings(items)
	return items
}

func displaySecretKey(rawKey string) string {
	normalizedKey := normalizeSecretStoreLookupKey(rawKey)
	if normalizedKey == "" {
		return ""
	}

	pathPart, keyPart, composite := splitStoredSecretPathKey(normalizedKey)
	if !composite {
		return normalizedKey
	}
	return "/" + pathPart + ":" + keyPart
}

func splitStoredSecretPathKey(value string) (string, string, bool) {
	index := strings.Index(value, ":")
	if index <= 0 {
		return "", "", false
	}

	pathPart := strings.Trim(strings.TrimSpace(value[:index]), "/")
	keyPart := strings.TrimSpace(value[index+1:])
	if pathPart == "" || keyPart == "" {
		return "", "", false
	}

	return pathPart, keyPart, true
}

func normalizeSecretStoreLookupKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	return strings.Trim(trimmed, "/")
}

func newMaskCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var input cliutil.InputFlags

	command := &cobra.Command{
		Use:   "mask",
		Short: "Mask secret values in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := cliutil.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			masked, err := secretProvider.MaskPayload(command.Context(), value)
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(command, outputFormat, masked, nil)
		},
	}

	cliutil.BindInputFlags(command, &input)
	return command
}

func newResolveCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var input cliutil.InputFlags

	command := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve secret placeholders in payload",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := cliutil.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			resolved, err := secretProvider.ResolvePayload(command.Context(), value)
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(command, outputFormat, resolved, nil)
		},
	}

	cliutil.BindInputFlags(command, &input)
	return command
}

func newNormalizeCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var input cliutil.InputFlags

	command := &cobra.Command{
		Use:   "normalize",
		Short: "Normalize secret placeholders",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			value, err := cliutil.DecodeInput[resource.Value](command, input)
			if err != nil {
				return err
			}

			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			normalized, err := secretProvider.NormalizeSecretPlaceholders(command.Context(), value)
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(command, outputFormat, normalized, nil)
		},
	}

	cliutil.BindInputFlags(command, &input)
	return command
}

func newDetectCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	var input cliutil.InputFlags
	var pathFlag string
	var fix bool
	var secretAttribute string

	command := &cobra.Command{
		Use:   "detect [path]",
		Short: "Detect potential secrets in payload or local resources",
		Example: strings.Join([]string{
			"  declarest secret detect /customers/",
			"  declarest secret detect --fix /customers/",
			"  declarest secret detect --secret-attribute /apiToken < payload.json",
			"  declarest secret detect --fix --path /customers/acme < payload.json",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			resolvedPath, err := cliutil.ResolvePathInput(pathFlag, args, false)
			if err != nil {
				return err
			}

			value, hasInput, err := decodeDetectInput(command, input)
			if err != nil {
				return err
			}

			secretProvider, err := cliutil.RequireSecretProvider(deps)
			if err != nil {
				return err
			}

			outputFormat, err := cliutil.ResolveContextOutputFormat(command.Context(), deps, globalFlags)
			if err != nil {
				return err
			}

			result, err := detectapp.Execute(command.Context(), detectapp.Dependencies{
				Orchestrator: deps.Orchestrator,
				Metadata:     deps.Services.MetadataService(),
				Secrets:      secretProvider,
			}, detectapp.Request{
				ResolvedPath:    resolvedPath,
				Value:           value,
				HasInput:        hasInput,
				Fix:             fix,
				SecretAttribute: secretAttribute,
			})
			if err != nil {
				return err
			}

			return cliutil.WriteOutput(command, outputFormat, result.Output, nil)
		},
	}

	cliutil.BindInputFlags(command, &input)
	cliutil.BindPathFlag(command, &pathFlag)
	cliutil.RegisterPathFlagCompletion(command, deps)
	command.ValidArgsFunction = cliutil.SinglePathArgCompletionFunc(deps)
	command.Flags().BoolVar(&fix, "fix", false, "write detected secret attributes to metadata")
	command.Flags().StringVar(&secretAttribute, "secret-attribute", "", "apply only one detected JSON pointer attribute")
	return command
}

func decodeDetectInput(command *cobra.Command, flags cliutil.InputFlags) (resource.Value, bool, error) {
	data, err := cliutil.ReadInput(command, flags)
	if err != nil {
		if isInputRequiredError(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var value resource.Value
	switch resolveSecretInputPayloadType(flags.ContentType, flags.Payload) {
	case cliutil.OutputJSON:
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, false, cliutil.ValidationError("invalid json input", err)
		}
	case cliutil.OutputYAML:
		if err := yaml.Unmarshal(data, &value); err != nil {
			return nil, false, cliutil.ValidationError("invalid yaml input", err)
		}
	default:
		return nil, false, cliutil.ValidationError("invalid input content type: use json, yaml, application/json, or application/yaml", nil)
	}

	return value, true, nil
}

func resolveSecretInputPayloadType(contentType string, payloadPath string) string {
	normalized := strings.ToLower(strings.TrimSpace(contentType))
	switch normalized {
	case "json", "application/json":
		return cliutil.OutputJSON
	case "yaml", "application/yaml", "text/yaml", "application/x-yaml", "text/x-yaml":
		return cliutil.OutputYAML
	}

	lowerPath := strings.ToLower(strings.TrimSpace(payloadPath))
	switch {
	case strings.HasSuffix(lowerPath, ".json"):
		return cliutil.OutputJSON
	case strings.HasSuffix(lowerPath, ".yaml"), strings.HasSuffix(lowerPath, ".yml"):
		return cliutil.OutputYAML
	default:
		return cliutil.OutputJSON
	}
}

func isInputRequiredError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "input is required")
}
