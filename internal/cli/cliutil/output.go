package cliutil

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/commandmeta"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

const (
	OutputAuto = "auto"
	OutputText = "text"
	OutputJSON = "json"
	OutputYAML = "yaml"
)

func ValidateOutputFormat(format string) error {
	switch format {
	case OutputAuto, OutputText, OutputJSON, OutputYAML:
		return nil
	default:
		return ValidationError("invalid output format: use auto, text, json, or yaml", nil)
	}
}

func ValidateOutputFormatForCommandPath(commandPath string, format string) error {
	switch strings.TrimSpace(format) {
	case "", OutputAuto, OutputText:
		return nil
	}

	switch commandmeta.OutputPolicyForPath(commandPath) {
	case commandmeta.OutputPolicyTextOnly:
		return ValidationError("command supports only text output; use --output text or --output auto", nil)
	case commandmeta.OutputPolicyYAMLDefaultTextOrYAML:
		if strings.TrimSpace(format) == OutputYAML {
			return nil
		}
		return ValidationError("command supports only yaml or text output; use --output yaml, text, or auto", nil)
	default:
		return nil
	}
}

func ResolveContextOutputFormat(ctx context.Context, deps CommandDependencies, globalFlags *GlobalFlags) (string, error) {
	if globalFlags == nil || globalFlags.Output == "" {
		return OutputJSON, nil
	}
	if globalFlags.Output != OutputAuto {
		return globalFlags.Output, nil
	}
	if deps.Contexts == nil {
		return OutputJSON, nil
	}

	resolvedContext, err := deps.Contexts.ResolveContext(ctx, configdomain.ContextSelection{Name: globalFlags.Context})
	if err != nil {
		return "", err
	}

	switch resolvedContext.Repository.ResourceFormat {
	case "", configdomain.ResourceFormatJSON:
		return OutputJSON, nil
	case configdomain.ResourceFormatYAML:
		return OutputYAML, nil
	default:
		return OutputJSON, nil
	}
}

func WriteOutput[T any](command *cobra.Command, format string, value T, renderText func(io.Writer, T) error) error {
	if isNilOutputValue(value) {
		return nil
	}

	switch format {
	case OutputAuto:
		if bytesValue, ok := resource.BinaryBytes(any(value)); ok {
			_, err := command.OutOrStdout().Write(bytesValue)
			return err
		}
		if containsNestedBinaryValue(any(value)) {
			return ValidationError("binary collections require --output json or yaml", nil)
		}
		if textValue, ok := any(value).(string); ok {
			_, err := io.WriteString(command.OutOrStdout(), textValue)
			return err
		}
		if renderText != nil {
			return renderText(command.OutOrStdout(), value)
		}
		_, err := fmt.Fprintln(command.OutOrStdout(), value)
		return err
	case OutputText:
		if bytesValue, ok := resource.BinaryBytes(any(value)); ok {
			_, err := command.OutOrStdout().Write(bytesValue)
			return err
		}
		if containsNestedBinaryValue(any(value)) {
			return ValidationError("binary collections require --output json or yaml", nil)
		}
		if renderText != nil {
			return renderText(command.OutOrStdout(), value)
		}
		if textValue, ok := any(value).(string); ok {
			_, err := fmt.Fprintln(command.OutOrStdout(), textValue)
			return err
		}
		_, err := fmt.Fprintln(command.OutOrStdout(), value)
		return err
	case OutputJSON:
		prepared := any(value)
		if containsNestedBinaryValue(any(value)) {
			var err error
			prepared, err = prepareStructuredOutputValue(any(value))
			if err != nil {
				return err
			}
		}
		encoded, err := json.MarshalIndent(prepared, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(command.OutOrStdout(), string(encoded))
		return err
	case OutputYAML:
		prepared := any(value)
		if containsNestedBinaryValue(any(value)) {
			var err error
			prepared, err = prepareStructuredOutputValue(any(value))
			if err != nil {
				return err
			}
		}
		encoded, err := yaml.Marshal(prepared)
		if err != nil {
			return err
		}
		_, err = fmt.Fprint(command.OutOrStdout(), string(encoded))
		return err
	default:
		return ValidationError("invalid output format: use auto, text, json, or yaml", nil)
	}
}

func ResolvePayloadAwareOutputFormat(
	ctx context.Context,
	deps CommandDependencies,
	globalFlags *GlobalFlags,
	value any,
) (string, error) {
	format, err := ResolveContextOutputFormat(ctx, deps, globalFlags)
	if err != nil {
		return "", err
	}
	if globalFlags == nil || globalFlags.Output != OutputAuto {
		return format, nil
	}
	if _, ok := resource.BinaryBytes(value); ok {
		return OutputAuto, nil
	}
	if _, ok := value.(string); ok {
		return OutputAuto, nil
	}
	if containsNestedBinaryValue(value) {
		return OutputAuto, nil
	}
	return format, nil
}

func WriteText(command *cobra.Command, format string, text string) error {
	return WriteOutput(command, format, text, func(w io.Writer, value string) error {
		_, err := fmt.Fprintln(w, value)
		return err
	})
}

func isNilOutputValue[T any](value T) bool {
	anyValue := any(value)
	if anyValue == nil {
		return true
	}

	reflected := reflect.ValueOf(anyValue)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

type binaryStructuredOutput struct {
	Encoding  string `json:"encoding" yaml:"encoding"`
	MediaType string `json:"mediaType" yaml:"mediaType"`
	Data      string `json:"data" yaml:"data"`
}

func containsNestedBinaryValue(value any) bool {
	if value == nil {
		return false
	}
	if resource.IsBinaryValue(value) {
		return true
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Pointer, reflect.Interface:
		if reflected.IsNil() {
			return false
		}
		return containsNestedBinaryValue(reflected.Elem().Interface())
	case reflect.Slice, reflect.Array:
		for idx := 0; idx < reflected.Len(); idx++ {
			if containsNestedBinaryValue(reflected.Index(idx).Interface()) {
				return true
			}
		}
	case reflect.Map:
		for _, key := range reflected.MapKeys() {
			if containsNestedBinaryValue(reflected.MapIndex(key).Interface()) {
				return true
			}
		}
	case reflect.Struct:
		for idx := 0; idx < reflected.NumField(); idx++ {
			if !reflected.Type().Field(idx).IsExported() {
				continue
			}
			if containsNestedBinaryValue(reflected.Field(idx).Interface()) {
				return true
			}
		}
	}

	return false
}

func prepareStructuredOutputValue(value any) (any, error) {
	if bytesValue, ok := resource.BinaryBytes(value); ok {
		return binaryStructuredOutput{
			Encoding:  "base64",
			MediaType: "application/octet-stream",
			Data:      base64.StdEncoding.EncodeToString(bytesValue),
		}, nil
	}
	if value == nil {
		return nil, nil
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Pointer, reflect.Interface:
		if reflected.IsNil() {
			return nil, nil
		}
		return prepareStructuredOutputValue(reflected.Elem().Interface())
	case reflect.Slice, reflect.Array:
		items := make([]any, reflected.Len())
		for idx := 0; idx < reflected.Len(); idx++ {
			item, err := prepareStructuredOutputValue(reflected.Index(idx).Interface())
			if err != nil {
				return nil, err
			}
			items[idx] = item
		}
		return items, nil
	case reflect.Map:
		if reflected.Type().Key().Kind() != reflect.String {
			return value, nil
		}
		items := make(map[string]any, reflected.Len())
		for _, key := range reflected.MapKeys() {
			item, err := prepareStructuredOutputValue(reflected.MapIndex(key).Interface())
			if err != nil {
				return nil, err
			}
			items[key.String()] = item
		}
		return items, nil
	case reflect.Struct:
		items := make(map[string]any, reflected.NumField())
		for idx := 0; idx < reflected.NumField(); idx++ {
			field := reflected.Type().Field(idx)
			if !field.IsExported() {
				continue
			}
			item, err := prepareStructuredOutputValue(reflected.Field(idx).Interface())
			if err != nil {
				return nil, err
			}
			items[field.Name] = item
		}
		return items, nil
	default:
		return value, nil
	}
}
