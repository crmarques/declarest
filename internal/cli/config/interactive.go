package config

import (
	"fmt"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func resolveCreateContextInput(command *cobra.Command, input common.InputFlags, prompter configPrompter) (configdomain.Context, error) {
	if shouldUseInteractiveCreate(command, input, prompter) {
		return promptCreateContext(command, prompter)
	}
	return decodeContextStrict(command, input)
}

func shouldUseInteractiveCreate(command *cobra.Command, input common.InputFlags, prompter configPrompter) bool {
	if input.File != "" {
		return false
	}
	if common.HasPipedInput(command) {
		return false
	}
	return prompter.IsInteractive(command)
}

func promptCreateContext(command *cobra.Command, prompter configPrompter) (configdomain.Context, error) {
	name, err := prompter.Input(command, "Context name: ", true)
	if err != nil {
		return configdomain.Context{}, err
	}

	repositoryType, err := prompter.Select(command, "Select repository type", []string{"filesystem", "git"})
	if err != nil {
		return configdomain.Context{}, err
	}

	baseDirPrompt := "Repository base-dir: "
	if repositoryType == "git" {
		baseDirPrompt = "Git local base-dir: "
	}
	baseDir, err := prompter.Input(command, baseDirPrompt, true)
	if err != nil {
		return configdomain.Context{}, err
	}

	metadataPrompt := fmt.Sprintf("Metadata base-dir (defaults to %s): ", baseDir)
	metadataBaseDir, err := prompter.Input(command, metadataPrompt, false)
	if err != nil {
		return configdomain.Context{}, err
	}
	if metadataBaseDir == "" {
		metadataBaseDir = baseDir
	}

	contextCfg := configdomain.Context{
		Name: name,
		Repository: configdomain.Repository{
			ResourceFormat: configdomain.ResourceFormatYAML,
		},
	}

	switch repositoryType {
	case "git":
		contextCfg.Repository.Git = &configdomain.GitRepository{
			Local: configdomain.GitLocal{BaseDir: baseDir},
		}
	default:
		contextCfg.Repository.Filesystem = &configdomain.FilesystemRepository{BaseDir: baseDir}
	}

	contextCfg.Metadata.BaseDir = metadataBaseDir

	return contextCfg, nil
}

func selectContextForAction(
	command *cobra.Command,
	contexts configdomain.ContextService,
	prompter configPrompter,
	actionLabel string,
) (string, error) {
	items, err := contexts.List(command.Context())
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", common.ValidationError("no contexts available", nil)
	}
	if !prompter.IsInteractive(command) {
		return "", common.ValidationError(fmt.Sprintf("context name is required: declarest config %s <name>", actionLabel), nil)
	}

	options := make([]string, 0, len(items))
	for _, item := range items {
		options = append(options, item.Name)
	}
	return prompter.Select(command, "Choose context", options)
}
