package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/spf13/cobra"
)

func NewCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	return newCommandWithPrompter(deps, globalFlags, terminalPrompter{})
}

func newCommandWithPrompter(
	deps common.CommandDependencies,
	globalFlags *common.GlobalFlags,
	prompter configPrompter,
) *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "Manage contexts",
		Args:  cobra.NoArgs,
	}

	command.AddCommand(
		newPrintTemplateCommand(),
		newAddCommand(deps, globalFlags, prompter),
		newEditCommand(deps, globalFlags),
		newUpdateCommand(deps),
		newDeleteCommand(deps, prompter),
		newRenameCommand(deps, prompter),
		newListCommand(deps, globalFlags),
		newUseCommand(deps, prompter),
		newShowCommand(deps, globalFlags, prompter),
		newCurrentCommand(deps, globalFlags),
		newResolveCommand(deps, globalFlags),
		newCheckCommand(deps, globalFlags),
		newValidateCommand(deps),
	)

	return command
}

func newPrintTemplateCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "print-template",
		Short: "Print a full context YAML template with guidance comments",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			_, err := io.WriteString(command.OutOrStdout(), contextTemplateYAML)
			return err
		},
	}
}

type addContextSelection struct {
	Contexts   []configdomain.Context
	CurrentCtx string
}

func newAddCommand(
	deps common.CommandDependencies,
	globalFlags *common.GlobalFlags,
	prompter configPrompter,
) *cobra.Command {
	var input common.InputFlags
	var contextName string
	var setCurrent bool

	command := &cobra.Command{
		Use:   "add [new-context-name]",
		Short: "Add contexts from input or create one interactively",
		Example: strings.Join([]string{
			"  declarest config add --file context.yaml",
			"  declarest config add --file contexts.yaml --context-name prod",
			"  cat contexts.yaml | declarest config add --set-current",
			"  declarest config add dev",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			contextArgName, err := resolveCreateContextName(args, selectedContextName(globalFlags))
			if err != nil {
				return err
			}

			effectiveImportContextName := strings.TrimSpace(contextName)
			if effectiveImportContextName != "" && contextArgName != "" && effectiveImportContextName != contextArgName {
				return common.ValidationError(
					fmt.Sprintf(
						"context name conflict: positional/--context %q differs from --context-name %q",
						contextArgName,
						effectiveImportContextName,
					),
					nil,
				)
			}
			if effectiveImportContextName == "" {
				effectiveImportContextName = contextArgName
			}

			if shouldUseInteractiveCreate(command, input, prompter) {
				cfg, err := resolveCreateContextInput(command, input, prompter, effectiveImportContextName)
				if err != nil {
					return err
				}
				if err := contexts.Create(command.Context(), cfg); err != nil {
					return err
				}
				if setCurrent {
					return contexts.SetCurrent(command.Context(), cfg.Name)
				}
				return nil
			}

			decoded, err := decodeContextImportInputStrict(command, input)
			if err != nil {
				return err
			}

			selection, err := selectContextsForAdd(decoded, effectiveImportContextName)
			if err != nil {
				return err
			}

			currentName := ""
			if setCurrent {
				currentName, err = resolveSetCurrentContext(selection)
				if err != nil {
					return err
				}
			}

			if err := validateAddTargets(command, contexts, selection.Contexts); err != nil {
				return err
			}

			for _, cfg := range selection.Contexts {
				if err := contexts.Create(command.Context(), cfg); err != nil {
					return err
				}
			}

			if !setCurrent {
				return nil
			}

			return contexts.SetCurrent(command.Context(), currentName)
		},
	}

	command.Flags().StringVarP(&input.Payload, "payload", "f", "", "payload file path (use '-' to read object from stdin)")
	command.Flags().StringVar(&input.Payload, "file", "", "legacy alias for --payload")
	_ = command.Flags().MarkHidden("file")
	command.Flags().StringVarP(&input.Format, "format", "i", common.OutputYAML, "input format: json|yaml")
	command.Flags().StringVar(&contextName, "context-name", "", "context name to import (catalog) or assign (single context)")
	command.Flags().BoolVar(&setCurrent, "set-current", false, "set imported context as current")
	common.RegisterInputFormatFlagCompletion(command)
	return command
}

func selectContextsForAdd(input contextImportInput, contextName string) (addContextSelection, error) {
	trimmedContextName := strings.TrimSpace(contextName)
	switch input.Kind {
	case contextImportInputContext:
		cfg := input.Context
		if trimmedContextName != "" {
			cfg.Name = trimmedContextName
		}
		return addContextSelection{
			Contexts: []configdomain.Context{cfg},
		}, nil
	case contextImportInputCatalog:
		if len(input.Catalog.Contexts) == 0 {
			return addContextSelection{}, common.ValidationError("input context catalog has no contexts", nil)
		}

		if trimmedContextName == "" {
			contexts := make([]configdomain.Context, len(input.Catalog.Contexts))
			copy(contexts, input.Catalog.Contexts)
			return addContextSelection{
				Contexts:   contexts,
				CurrentCtx: strings.TrimSpace(input.Catalog.CurrentCtx),
			}, nil
		}

		for _, item := range input.Catalog.Contexts {
			if item.Name == trimmedContextName {
				return addContextSelection{
					Contexts: []configdomain.Context{item},
				}, nil
			}
		}

		return addContextSelection{}, common.ValidationError(
			fmt.Sprintf("context %q not found in input catalog", trimmedContextName),
			nil,
		)
	default:
		return addContextSelection{}, common.ValidationError("unsupported config input shape", nil)
	}
}

func resolveSetCurrentContext(selection addContextSelection) (string, error) {
	if len(selection.Contexts) == 1 {
		return selection.Contexts[0].Name, nil
	}

	if selection.CurrentCtx != "" {
		for _, item := range selection.Contexts {
			if item.Name == selection.CurrentCtx {
				return selection.CurrentCtx, nil
			}
		}
		return "", common.ValidationError(
			fmt.Sprintf("input current-ctx %q is not present in imported contexts", selection.CurrentCtx),
			nil,
		)
	}

	return "", common.ValidationError(
		"set-current requires a single imported context or a catalog current-ctx value",
		nil,
	)
}

func resolveCreateContextName(args []string, contextFlagName string) (string, error) {
	positionalName := ""
	if len(args) > 0 {
		positionalName = strings.TrimSpace(args[0])
	}

	flagName := strings.TrimSpace(contextFlagName)
	if positionalName != "" && flagName != "" && positionalName != flagName {
		return "", common.ValidationError(
			fmt.Sprintf("context name conflict: positional %q differs from --context %q", positionalName, flagName),
			nil,
		)
	}

	if positionalName != "" {
		return positionalName, nil
	}
	return flagName, nil
}

func validateAddTargets(command *cobra.Command, contexts configdomain.ContextService, items []configdomain.Context) error {
	if len(items) == 0 {
		return common.ValidationError("no contexts found in input", nil)
	}

	existing, err := contexts.List(command.Context())
	if err != nil {
		return err
	}

	existingNames := make(map[string]struct{}, len(existing))
	for _, item := range existing {
		existingNames[item.Name] = struct{}{}
	}

	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			return common.ValidationError("context name is required", nil)
		}
		if _, duplicated := seen[name]; duplicated {
			return common.ValidationError(fmt.Sprintf("input contains duplicate context %q", name), nil)
		}
		if _, exists := existingNames[name]; exists {
			return common.ValidationError(fmt.Sprintf("context %q already exists", name), nil)
		}
		seen[name] = struct{}{}
	}

	return nil
}

func newUpdateCommand(deps common.CommandDependencies) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "update",
		Short: "Update a context from input",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			cfg, err := decodeContextStrict(command, input)
			if err != nil {
				return err
			}
			return contexts.Update(command.Context(), cfg)
		},
	}

	command.Flags().StringVarP(&input.Payload, "payload", "f", "", "payload file path (use '-' to read object from stdin)")
	command.Flags().StringVar(&input.Payload, "file", "", "legacy alias for --payload")
	_ = command.Flags().MarkHidden("file")
	command.Flags().StringVarP(&input.Format, "format", "i", common.OutputYAML, "input format: json|yaml")
	common.RegisterInputFormatFlagCompletion(command)
	return command
}

func newDeleteCommand(deps common.CommandDependencies, prompter configPrompter) *cobra.Command {
	command := &cobra.Command{
		Use:   "delete [name]",
		Short: "Delete a context (interactive when name is omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			name := ""
			if len(args) > 0 {
				name = args[0]
			} else {
				selected, err := selectContextForAction(command, contexts, prompter, "delete")
				if err != nil {
					return err
				}
				confirmed, err := prompter.Confirm(command, fmt.Sprintf("Delete context %q?", selected), false)
				if err != nil {
					return err
				}
				if !confirmed {
					return common.WriteText(command, common.OutputText, "delete canceled")
				}
				name = selected
			}
			return contexts.Delete(command.Context(), name)
		},
	}
	registerSingleContextArgCompletion(command, deps)
	return command
}

func newRenameCommand(deps common.CommandDependencies, prompter configPrompter) *cobra.Command {
	command := &cobra.Command{
		Use:   "rename [from] [to]",
		Short: "Rename a context (interactive when args are omitted)",
		Args:  cobra.MaximumNArgs(2),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			fromName := ""
			toName := ""
			switch len(args) {
			case 2:
				fromName = args[0]
				toName = args[1]
			case 1:
				fromName = args[0]
				if !prompter.IsInteractive(command) {
					return common.ValidationError("new context name is required", nil)
				}
				toName, err = prompter.Input(command, "New context name: ", true)
				if err != nil {
					return err
				}
			default:
				fromName, err = selectContextForAction(command, contexts, prompter, "rename")
				if err != nil {
					return err
				}
				toName, err = prompter.Input(command, "New context name: ", true)
				if err != nil {
					return err
				}
			}

			return contexts.Rename(command.Context(), fromName, toName)
		},
	}
	registerRenameFromArgCompletion(command, deps)
	return command
}

func newListCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List contexts",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			items, err := contexts.List(command.Context())
			if err != nil {
				return err
			}
			return common.WriteOutput(command, globalFlags.Output, items, func(w io.Writer, value []configdomain.Context) error {
				for _, item := range value {
					if _, writeErr := fmt.Fprintln(w, item.Name); writeErr != nil {
						return writeErr
					}
				}
				return nil
			})
		},
	}
}

func newUseCommand(deps common.CommandDependencies, prompter configPrompter) *cobra.Command {
	command := &cobra.Command{
		Use:   "use [name]",
		Short: "Set current context (interactive when name is omitted)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			name := ""
			if len(args) > 0 {
				name = args[0]
			} else {
				name, err = selectContextForAction(command, contexts, prompter, "use")
				if err != nil {
					return err
				}
			}
			return contexts.SetCurrent(command.Context(), name)
		},
	}
	registerSingleContextArgCompletion(command, deps)
	return command
}

func newShowCommand(
	deps common.CommandDependencies,
	globalFlags *common.GlobalFlags,
	prompter configPrompter,
) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Show a context from --context or interactive selection",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			name := ""
			if globalFlags != nil {
				name = strings.TrimSpace(globalFlags.Context)
			}
			if name == "" {
				name, err = selectContextForAction(command, contexts, prompter, "show --context")
				if err != nil {
					return err
				}
			}

			shown, err := contexts.ResolveContext(command.Context(), configdomain.ContextSelection{Name: name})
			if err != nil {
				return err
			}

			return common.WriteOutput(command, common.OutputYAML, shown, nil)
		},
	}
}

func newCurrentCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Get current context",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			current, err := contexts.GetCurrent(command.Context())
			if err != nil {
				return err
			}
			return common.WriteOutput(command, globalFlags.Output, current, func(w io.Writer, value configdomain.Context) error {
				_, writeErr := fmt.Fprintln(w, value.Name)
				return writeErr
			})
		},
	}
}

func newResolveCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	var overrides []string

	command := &cobra.Command{
		Use:   "resolve",
		Short: "Resolve active context with overrides",
		Example: strings.Join([]string{
			"  declarest config resolve",
			"  declarest config resolve --context prod",
			"  declarest config resolve --set resource-server.http.base-url=https://api.example.com",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			overridesMap, err := parseOverrides(overrides)
			if err != nil {
				return err
			}

			resolved, err := contexts.ResolveContext(command.Context(), configdomain.ContextSelection{
				Name:      globalFlags.Context,
				Overrides: overridesMap,
			})
			if err != nil {
				return err
			}

			return common.WriteOutput(command, globalFlags.Output, resolved, func(w io.Writer, value configdomain.Context) error {
				_, writeErr := fmt.Fprintln(w, value.Name)
				return writeErr
			})
		},
	}

	command.Flags().StringArrayVarP(&overrides, "set", "e", nil, "override key=value (repeatable)")
	return command
}

func newValidateCommand(deps common.CommandDependencies) *cobra.Command {
	var input common.InputFlags

	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate a context from input",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}
			cfg, err := decodeContextStrict(command, input)
			if err != nil {
				return err
			}
			return contexts.Validate(command.Context(), cfg)
		},
	}

	command.Flags().StringVarP(&input.Payload, "payload", "f", "", "payload file path (use '-' to read object from stdin)")
	command.Flags().StringVar(&input.Payload, "file", "", "legacy alias for --payload")
	_ = command.Flags().MarkHidden("file")
	command.Flags().StringVarP(&input.Format, "format", "i", common.OutputYAML, "input format: json|yaml")
	common.RegisterInputFormatFlagCompletion(command)
	return command
}

func newCheckCommand(deps common.CommandDependencies, globalFlags *common.GlobalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Check configured component availability and connectivity",
		Example: strings.Join([]string{
			"  declarest config check",
			"  declarest --context prod config check --output json",
		}, "\n"),
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			contexts, err := common.RequireContexts(deps)
			if err != nil {
				return err
			}

			resolvedContext, err := contexts.ResolveContext(command.Context(), configdomain.ContextSelection{
				Name: selectedContextName(globalFlags),
			})
			if err != nil {
				return err
			}

			report := runConfigCheck(command, deps, resolvedContext)
			if err := common.WriteOutput(command, selectedOutputFormat(globalFlags), report, renderConfigCheckText); err != nil {
				return err
			}

			if report.Summary.Fail > 0 {
				return common.ValidationError(
					fmt.Sprintf("config check failed for context %q: %d component(s) unavailable", report.Context, report.Summary.Fail),
					nil,
				)
			}
			return nil
		},
	}
}

type configCheckStatus string

const (
	configCheckOK   configCheckStatus = "ok"
	configCheckWarn configCheckStatus = "warn"
	configCheckFail configCheckStatus = "fail"
	configCheckSkip configCheckStatus = "skip"
)

type configCheckResult struct {
	Component string            `json:"component" yaml:"component"`
	Status    configCheckStatus `json:"status" yaml:"status"`
	Details   string            `json:"details,omitempty" yaml:"details,omitempty"`
	Error     string            `json:"error,omitempty" yaml:"error,omitempty"`
}

type configCheckSummary struct {
	OK   int `json:"ok" yaml:"ok"`
	Warn int `json:"warn" yaml:"warn"`
	Fail int `json:"fail" yaml:"fail"`
	Skip int `json:"skip" yaml:"skip"`
}

type configCheckReport struct {
	Context    string              `json:"context" yaml:"context"`
	Passed     bool                `json:"passed" yaml:"passed"`
	Summary    configCheckSummary  `json:"summary" yaml:"summary"`
	Components []configCheckResult `json:"components" yaml:"components"`
}

func runConfigCheck(command *cobra.Command, deps common.CommandDependencies, cfg configdomain.Context) configCheckReport {
	items := []configCheckResult{
		{
			Component: "context",
			Status:    configCheckOK,
			Details:   "context resolved successfully",
		},
		checkRepository(command, deps, cfg),
		checkMetadata(command, deps, cfg),
		checkResourceServer(command, deps, cfg),
		checkSecretStore(command, deps, cfg),
	}

	summary := configCheckSummary{}
	for _, item := range items {
		switch item.Status {
		case configCheckOK:
			summary.OK++
		case configCheckWarn:
			summary.Warn++
		case configCheckFail:
			summary.Fail++
		case configCheckSkip:
			summary.Skip++
		}
	}

	return configCheckReport{
		Context:    cfg.Name,
		Passed:     summary.Fail == 0,
		Summary:    summary,
		Components: items,
	}
}

func checkRepository(command *cobra.Command, deps common.CommandDependencies, cfg configdomain.Context) configCheckResult {
	result := configCheckResult{
		Component: "repository",
	}

	repositoryService, err := common.RequireRepositorySync(deps)
	if err != nil {
		result.Status = configCheckFail
		result.Error = err.Error()
		return result
	}

	if err := repositoryService.Check(command.Context()); err != nil {
		result.Status = configCheckFail
		result.Error = err.Error()
		return result
	}

	switch {
	case cfg.Repository.Filesystem != nil:
		result.Status = configCheckOK
		result.Details = "filesystem repository is accessible"
		return result
	case cfg.Repository.Git != nil && cfg.Repository.Git.Remote != nil:
		status, err := repositoryService.SyncStatus(command.Context())
		if err != nil {
			result.Status = configCheckFail
			result.Error = err.Error()
			return result
		}
		result.Status = configCheckOK
		result.Details = fmt.Sprintf("git repository is accessible (state=%s ahead=%d behind=%d)", status.State, status.Ahead, status.Behind)
		return result
	case cfg.Repository.Git != nil:
		result.Status = configCheckOK
		result.Details = "git repository is accessible (remote not configured)"
		return result
	default:
		result.Status = configCheckFail
		result.Error = "repository configuration is missing"
		return result
	}
}

func checkMetadata(command *cobra.Command, deps common.CommandDependencies, cfg configdomain.Context) configCheckResult {
	result := configCheckResult{
		Component: "metadata",
	}

	metadataService, err := common.RequireMetadataService(deps)
	if err != nil {
		result.Status = configCheckFail
		result.Error = err.Error()
		return result
	}

	if _, err := metadataService.ResolveForPath(command.Context(), "/"); err != nil {
		result.Status = configCheckFail
		result.Error = err.Error()
		return result
	}

	baseDir := strings.TrimSpace(cfg.Metadata.BaseDir)
	if baseDir == "" {
		result.Status = configCheckFail
		result.Error = "metadata.base-dir is empty"
		return result
	}

	info, err := os.Stat(baseDir)
	if err != nil {
		result.Status = configCheckFail
		result.Error = fmt.Sprintf("metadata base-dir check failed: %v", err)
		return result
	}
	if !info.IsDir() {
		result.Status = configCheckFail
		result.Error = "metadata base-dir is not a directory"
		return result
	}

	result.Status = configCheckOK
	result.Details = "metadata service is accessible"
	return result
}

func checkResourceServer(command *cobra.Command, deps common.CommandDependencies, cfg configdomain.Context) configCheckResult {
	result := configCheckResult{
		Component: "resource-server",
	}

	if cfg.ResourceServer == nil {
		result.Status = configCheckSkip
		result.Details = "not configured"
		return result
	}

	orchestratorService, err := common.RequireOrchestrator(deps)
	if err != nil {
		result.Status = configCheckFail
		result.Error = err.Error()
		return result
	}

	_, err = orchestratorService.ListRemote(command.Context(), "/", orchestratordomain.ListPolicy{Recursive: false})
	if err == nil {
		result.Status = configCheckOK
		result.Details = "resource server probe succeeded"
		return result
	}

	switch typedCategory(err) {
	case faults.NotFoundError, faults.ValidationError, faults.ConflictError:
		result.Status = configCheckWarn
		result.Details = fmt.Sprintf("probe reached server but returned %s", typedCategory(err))
		result.Error = err.Error()
		return result
	default:
		result.Status = configCheckFail
		result.Error = err.Error()
		return result
	}
}

func checkSecretStore(command *cobra.Command, deps common.CommandDependencies, cfg configdomain.Context) configCheckResult {
	result := configCheckResult{
		Component: "secret-store",
	}

	if cfg.SecretStore == nil {
		result.Status = configCheckSkip
		result.Details = "not configured"
		return result
	}

	secretProvider, err := common.RequireSecretProvider(deps)
	if err != nil {
		result.Status = configCheckFail
		result.Error = err.Error()
		return result
	}

	keys, err := secretProvider.List(command.Context())
	if err != nil {
		result.Status = configCheckFail
		result.Error = err.Error()
		return result
	}

	result.Status = configCheckOK
	result.Details = fmt.Sprintf("secret store is accessible (keys=%d)", len(keys))
	return result
}

func renderConfigCheckText(writer io.Writer, report configCheckReport) error {
	if _, err := fmt.Fprintf(writer, "Config check for context %q\n", report.Context); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(writer, strings.Repeat("-", 80)); err != nil {
		return err
	}

	for _, item := range report.Components {
		line := fmt.Sprintf("[%s] %-14s %s", strings.ToUpper(string(item.Status)), item.Component, item.Details)
		if strings.TrimSpace(item.Details) == "" {
			line = fmt.Sprintf("[%s] %-14s", strings.ToUpper(string(item.Status)), item.Component)
		}
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
		if strings.TrimSpace(item.Error) != "" {
			if _, err := fmt.Fprintf(writer, "       %-14s %s\n", "error:", item.Error); err != nil {
				return err
			}
		}
	}

	if _, err := fmt.Fprintln(writer, strings.Repeat("-", 80)); err != nil {
		return err
	}

	state := "PASS"
	if !report.Passed {
		state = "FAIL"
	}

	_, err := fmt.Fprintf(
		writer,
		"Result: %s (ok=%d warn=%d fail=%d skip=%d)\n",
		state,
		report.Summary.OK,
		report.Summary.Warn,
		report.Summary.Fail,
		report.Summary.Skip,
	)
	return err
}

func selectedContextName(globalFlags *common.GlobalFlags) string {
	if globalFlags == nil {
		return ""
	}
	return strings.TrimSpace(globalFlags.Context)
}

func selectedOutputFormat(globalFlags *common.GlobalFlags) string {
	if globalFlags == nil || strings.TrimSpace(globalFlags.Output) == "" {
		return common.OutputAuto
	}
	return globalFlags.Output
}

func typedCategory(err error) faults.ErrorCategory {
	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		return ""
	}
	return typedErr.Category
}

func parseOverrides(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}

	parsed := make(map[string]string, len(values))
	for _, value := range values {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			return nil, common.ValidationError("invalid override: expected key=value", nil)
		}
		parsed[strings.TrimSpace(parts[0])] = parts[1]
	}

	return parsed, nil
}
