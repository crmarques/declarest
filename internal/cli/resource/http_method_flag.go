package resource

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/metadata"
	"github.com/spf13/cobra"
)

var httpMethodCompletionValues = []string{
	"CONNECT",
	"DELETE",
	"GET",
	"HEAD",
	"OPTIONS",
	"PATCH",
	"POST",
	"PUT",
	"TRACE",
}

func bindHTTPMethodFlag(command *cobra.Command, httpMethod *string) {
	command.Flags().StringVar(httpMethod, "http-method", "", "override metadata operation HTTP method for remote server calls")
	_ = command.RegisterFlagCompletionFunc("http-method", func(
		_ *cobra.Command,
		_ []string,
		toComplete string,
	) ([]string, cobra.ShellCompDirective) {
		return common.CompleteValues(httpMethodCompletionValues, strings.ToUpper(toComplete))
	})
}

func validateHTTPMethodOverride(raw string) (string, bool, error) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return "", false, nil
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return "", false, common.ValidationError("flag --http-method must be a single HTTP method token", nil)
	}
	return value, true, nil
}

func applyHTTPMethodOverride(ctx context.Context, raw string, operations ...metadata.Operation) (context.Context, string, error) {
	method, hasOverride, err := validateHTTPMethodOverride(raw)
	if err != nil {
		return ctx, "", err
	}
	if !hasOverride {
		return ctx, "", nil
	}
	updated := ctx
	for _, operation := range operations {
		updated = metadata.WithOperationHTTPMethodOverride(updated, operation, method)
	}
	return updated, method, nil
}
