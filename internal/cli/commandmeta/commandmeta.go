package commandmeta

import (
	"strings"

	"github.com/spf13/cobra"
)

type PathCompletionSourceStrategy uint8

const (
	PathCompletionStrategyDefaultLocalFirstFallback PathCompletionSourceStrategy = iota
	PathCompletionStrategyRemoteFirstFallback
)

type OutputPolicy uint8

const (
	OutputPolicyStructured OutputPolicy = iota
	OutputPolicyTextOnly
	OutputPolicyYAMLDefaultTextOrYAML
)

func RequiresContextBootstrapPath(commandPath string) bool {
	normalized := strings.TrimSpace(commandPath)
	switch {
	case normalized == "declarest config check":
		return true
	case strings.HasPrefix(normalized, "declarest resource "):
		return true
	case strings.HasPrefix(normalized, "declarest metadata "):
		return true
	case strings.HasPrefix(normalized, "declarest repo "):
		return true
	case strings.HasPrefix(normalized, "declarest secret "):
		return true
	case strings.HasPrefix(normalized, "declarest resource-server "):
		return true
	}

	return false
}

func EmitsExecutionStatusPath(path string) bool {
	switch strings.TrimSpace(path) {
	case "declarest resource save",
		"declarest resource apply",
		"declarest resource create",
		"declarest resource update",
		"declarest resource delete":
		return true
	default:
		return false
	}
}

func OutputPolicyForPath(path string) OutputPolicy {
	switch strings.TrimSpace(path) {
	case "declarest config show":
		return OutputPolicyYAMLDefaultTextOrYAML
	case "declarest config print-template",
		"declarest secret get",
		"declarest resource-server check",
		"declarest resource-server get base-url",
		"declarest resource-server get token-url",
		"declarest resource-server get access-token",
		"declarest completion bash",
		"declarest completion zsh",
		"declarest completion fish",
		"declarest completion powershell":
		return OutputPolicyTextOnly
	default:
		return OutputPolicyStructured
	}
}

func PathCompletionSourceStrategyForCommand(command *cobra.Command) PathCompletionSourceStrategy {
	if command == nil {
		return PathCompletionStrategyDefaultLocalFirstFallback
	}

	switch parentCommandName(command) {
	case "resource":
		switch command.Name() {
		case "get", "save", "list", "delete":
			return PathCompletionStrategyRemoteFirstFallback
		}
	case "metadata", "secret":
		return PathCompletionStrategyDefaultLocalFirstFallback
	}

	return PathCompletionStrategyDefaultLocalFirstFallback
}

func parentCommandName(command *cobra.Command) string {
	if command == nil || command.Parent() == nil {
		return ""
	}
	return strings.TrimSpace(command.Parent().Name())
}
