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

package config

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	managedservicedomain "github.com/crmarques/declarest/managedservice"
	"github.com/spf13/cobra"
)

func newCheckCommand(deps cliutil.CommandDependencies, globalFlags *cliutil.GlobalFlags) *cobra.Command {
	command := &cobra.Command{
		Use:   "check [name]",
		Short: "Check configured component availability and connectivity",
		Example: strings.Join([]string{
			"  declarest context check",
			"  declarest context check prod",
			"  declarest --context prod context check --output json",
		}, "\n"),
		Args: cobra.MaximumNArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			contexts, err := cliutil.RequireContexts(deps)
			if err != nil {
				return err
			}
			contextName, err := resolveCreateContextName(args, selectedContextName(globalFlags))
			if err != nil {
				return err
			}

			resolvedContext, err := contexts.ResolveContext(command.Context(), configdomain.ContextSelection{
				Name: contextName,
			})
			if err != nil {
				return err
			}

			report := runConfigCheck(command, deps, resolvedContext)
			if err := cliutil.WriteOutput(command, cliutil.ResolveCommandOutputFormat(command, globalFlags), report, renderConfigCheckText); err != nil {
				return err
			}

			if report.Summary.Fail > 0 {
				return cliutil.ValidationError(
					fmt.Sprintf("context check failed for context %q: %d component(s) unavailable", report.Context, report.Summary.Fail),
					nil,
				)
			}
			return nil
		},
	}

	registerSingleContextArgCompletion(command, deps)
	return command
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

func runConfigCheck(command *cobra.Command, deps cliutil.CommandDependencies, cfg configdomain.Context) configCheckReport {
	items := []configCheckResult{
		{
			Component: "context",
			Status:    configCheckOK,
			Details:   "context resolved successfully",
		},
		checkRepository(command, deps, cfg),
		checkMetadata(command, deps, cfg),
		checkManagedService(command, deps, cfg),
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

func checkRepository(command *cobra.Command, deps cliutil.CommandDependencies, cfg configdomain.Context) configCheckResult {
	result := configCheckResult{
		Component: "repository",
	}

	repositoryService, err := cliutil.RequireRepositorySync(deps)
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

func checkMetadata(command *cobra.Command, deps cliutil.CommandDependencies, cfg configdomain.Context) configCheckResult {
	result := configCheckResult{
		Component: "metadata",
	}

	metadataService, err := cliutil.RequireMetadataService(deps)
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

	if strings.TrimSpace(cfg.Metadata.Bundle) != "" || strings.TrimSpace(cfg.Metadata.BundleFile) != "" {
		result.Status = configCheckOK
		result.Details = "metadata bundle is accessible"
		return result
	}

	baseDir := strings.TrimSpace(cfg.Metadata.BaseDir)
	if baseDir == "" {
		result.Status = configCheckFail
		result.Error = "metadata.baseDir is empty"
		return result
	}

	info, err := os.Stat(baseDir)
	if err != nil {
		result.Status = configCheckFail
		result.Error = fmt.Sprintf("metadata baseDir check failed: %v", err)
		return result
	}
	if !info.IsDir() {
		result.Status = configCheckFail
		result.Error = "metadata baseDir is not a directory"
		return result
	}

	result.Status = configCheckOK
	result.Details = "metadata service is accessible"
	return result
}

func checkManagedService(command *cobra.Command, deps cliutil.CommandDependencies, cfg configdomain.Context) configCheckResult {
	result := configCheckResult{
		Component: "managedService",
	}

	if cfg.ManagedService == nil {
		result.Status = configCheckSkip
		result.Details = "not configured"
		return result
	}

	managedServiceClient, err := cliutil.RequireManagedServiceClient(deps)
	if err != nil {
		result.Status = configCheckFail
		result.Error = err.Error()
		return result
	}

	probePath, err := resolveManagedServiceHealthCheckProbePath(cfg)
	if err != nil {
		result.Status = configCheckFail
		result.Error = err.Error()
		return result
	}

	_, err = managedServiceClient.Request(command.Context(), managedservicedomain.RequestSpec{
		Method: http.MethodGet,
		Path:   probePath,
	})
	if err == nil {
		result.Status = configCheckOK
		result.Details = fmt.Sprintf("managed service probe succeeded (GET %s)", renderManagedServiceHealthCheckTarget(cfg))
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

func renderManagedServiceHealthCheckTarget(cfg configdomain.Context) string {
	if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
		return "/"
	}
	healthCheck := strings.TrimSpace(cfg.ManagedService.HTTP.HealthCheck)
	if healthCheck != "" {
		return healthCheck
	}
	return strings.TrimSpace(cfg.ManagedService.HTTP.BaseURL)
}

func resolveManagedServiceHealthCheckProbePath(cfg configdomain.Context) (string, error) {
	if cfg.ManagedService == nil || cfg.ManagedService.HTTP == nil {
		return "/", nil
	}

	healthCheck := strings.TrimSpace(cfg.ManagedService.HTTP.HealthCheck)
	if healthCheck == "" {
		baseURL := strings.TrimSpace(cfg.ManagedService.HTTP.BaseURL)
		if baseURL == "" {
			return "/", nil
		}
		parsed, err := url.Parse(baseURL)
		if err != nil {
			return "", cliutil.ValidationError("managedService.http.url is invalid", err)
		}
		basePath := strings.TrimSpace(parsed.Path)
		if basePath == "" {
			return "/", nil
		}
		if !strings.HasPrefix(basePath, "/") {
			basePath = "/" + basePath
		}
		return basePath, nil
	}

	parsed, err := url.Parse(healthCheck)
	if err != nil {
		return "", cliutil.ValidationError("managedService.http.healthCheck is invalid", err)
	}
	if strings.TrimSpace(parsed.RawQuery) != "" {
		return "", cliutil.ValidationError("managedService.http.healthCheck must not include query parameters", nil)
	}
	if parsed.Scheme == "" && parsed.Host == "" {
		parsedPath := strings.TrimSpace(parsed.Path)
		if parsedPath == "" {
			return "", cliutil.ValidationError("managedService.http.healthCheck is invalid", nil)
		}
		if !strings.HasPrefix(parsedPath, "/") {
			parsedPath = "/" + parsedPath
		}
		return parsedPath, nil
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", cliutil.ValidationError("managedService.http.healthCheck URL must use http or https", nil)
	}
	if parsed.Host == "" {
		return "", cliutil.ValidationError("managedService.http.healthCheck URL host is required", nil)
	}
	baseParsed, err := url.Parse(strings.TrimSpace(cfg.ManagedService.HTTP.BaseURL))
	if err != nil {
		return "", cliutil.ValidationError("managedService.http.url is invalid", err)
	}
	if !strings.EqualFold(parsed.Scheme, baseParsed.Scheme) || !strings.EqualFold(parsed.Host, baseParsed.Host) {
		return "", cliutil.ValidationError(
			"managedService.http.healthCheck URL must share scheme and host with managedService.http.url",
			nil,
		)
	}
	parsedPath := strings.TrimSpace(parsed.Path)
	if parsedPath == "" {
		return "/", nil
	}
	if !strings.HasPrefix(parsedPath, "/") {
		parsedPath = "/" + parsedPath
	}
	return parsedPath, nil
}

func checkSecretStore(command *cobra.Command, deps cliutil.CommandDependencies, cfg configdomain.Context) configCheckResult {
	result := configCheckResult{
		Component: "secretStore",
	}

	if cfg.SecretStore == nil {
		result.Status = configCheckSkip
		result.Details = "not configured"
		return result
	}

	secretProvider, err := cliutil.RequireSecretProvider(deps)
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
