package commandmeta

import (
	"strings"

	"github.com/spf13/cobra"
)

const (
	AnnotationRequiresContextBootstrap = "declarest.io/requires-context-bootstrap"
	AnnotationEmitsExecutionStatus     = "declarest.io/emits-execution-status"
	AnnotationOutputPolicy             = "declarest.io/output-policy"
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
	OutputPolicyTextDefaultStructured
)

func RequiresContextBootstrap(command *cobra.Command) bool {
	return inheritedBoolAnnotation(command, AnnotationRequiresContextBootstrap)
}

func EmitsExecutionStatus(command *cobra.Command) bool {
	return inheritedBoolAnnotation(command, AnnotationEmitsExecutionStatus)
}

func OutputPolicyForCommand(command *cobra.Command) OutputPolicy {
	for current := command; current != nil; current = current.Parent() {
		if current.Annotations == nil {
			continue
		}

		switch strings.TrimSpace(current.Annotations[AnnotationOutputPolicy]) {
		case "text-only":
			return OutputPolicyTextOnly
		case "yaml-default-text-or-yaml":
			return OutputPolicyYAMLDefaultTextOrYAML
		case "text-default-structured":
			return OutputPolicyTextDefaultStructured
		}
	}

	return OutputPolicyStructured
}

func MarkRequiresContextBootstrap(command *cobra.Command) {
	setAnnotation(command, AnnotationRequiresContextBootstrap, "true")
}

func MarkEmitsExecutionStatus(command *cobra.Command) {
	setAnnotation(command, AnnotationEmitsExecutionStatus, "true")
}

func MarkTextOnlyOutput(command *cobra.Command) {
	setAnnotation(command, AnnotationOutputPolicy, "text-only")
}

func MarkYAMLDefaultTextOrYAMLOutput(command *cobra.Command) {
	setAnnotation(command, AnnotationOutputPolicy, "yaml-default-text-or-yaml")
}

func MarkTextDefaultStructuredOutput(command *cobra.Command) {
	setAnnotation(command, AnnotationOutputPolicy, "text-default-structured")
}

func setAnnotation(command *cobra.Command, key string, value string) {
	if command == nil {
		return
	}
	if command.Annotations == nil {
		command.Annotations = map[string]string{}
	}
	command.Annotations[key] = value
}

func inheritedBoolAnnotation(command *cobra.Command, key string) bool {
	for current := command; current != nil; current = current.Parent() {
		if current.Annotations == nil {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(current.Annotations[key]), "true") {
			return true
		}
	}
	return false
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
