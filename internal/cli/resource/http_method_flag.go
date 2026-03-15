// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resource

import (
	"context"
	"strings"

	"github.com/crmarques/declarest/internal/cli/cliutil"
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
		return cliutil.CompleteValues(httpMethodCompletionValues, strings.ToUpper(toComplete))
	})
}

func validateHTTPMethodOverride(raw string) (string, bool, error) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	if value == "" {
		return "", false, nil
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return "", false, cliutil.ValidationError("flag --http-method must be a single HTTP method token", nil)
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
