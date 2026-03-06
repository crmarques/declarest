package config

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	"github.com/spf13/cobra"
)

func TestCreateUpdateValidateRejectUnknownFields(t *testing.T) {
	t.Parallel()

	t.Run("create_rejects_unknown_json_field", func(t *testing.T) {
		t.Parallel()

		service := &testContextService{}
		_, err := executeConfigCommand(t, service, &cliutil.GlobalFlags{}, `{
  "name": "dev",
  "repository": {"filesystem": {"base-dir": "/tmp/repo"}},
  "unknown": true
}`, "add", "--format", "json")
		assertTypedCategory(t, err, faults.ValidationError)
		if service.createCalled {
			t.Fatal("expected create service call to be skipped on decode failure")
		}
	})

	t.Run("update_rejects_unknown_yaml_field", func(t *testing.T) {
		t.Parallel()

		service := &testContextService{}
		_, err := executeConfigCommand(t, service, &cliutil.GlobalFlags{}, `
name: dev
repository:
  filesystem:
    base-dir: /tmp/repo
unknown: true
`, "update", "--format", "yaml")
		assertTypedCategory(t, err, faults.ValidationError)
		if service.updateCalled {
			t.Fatal("expected update service call to be skipped on decode failure")
		}
	})

	t.Run("validate_rejects_unknown_json_nested_field", func(t *testing.T) {
		t.Parallel()

		service := &testContextService{}
		_, err := executeConfigCommand(t, service, &cliutil.GlobalFlags{}, `{
  "name": "dev",
  "repository": {"filesystem": {"base-dir": "/tmp/repo", "extra": true}}
}`, "validate")
		assertTypedCategory(t, err, faults.ValidationError)
		if service.validateCalled {
			t.Fatal("expected validate service call to be skipped on decode failure")
		}
	})
}

func TestPrintTemplateOutputsCommentedFullTemplateWithoutContextService(t *testing.T) {
	t.Parallel()

	output, err := executeConfigCommand(
		t,
		nil,
		&cliutil.GlobalFlags{},
		"",
		"print-template",
	)
	if err != nil {
		t.Fatalf("print-template returned error: %v", err)
	}

	requiredSnippets := []string{
		"contexts:",
		"current-ctx:",
		"repository:",
		"git:",
		"filesystem:",
		"managed-server:",
		"health-check:",
		"auth:",
		"proxy:",
		"http-url:",
		"https-url:",
		"no-proxy:",
		"oauth2:",
		"basic-auth:",
		"custom-headers:",
		"prefix: Bearer",
		"value: change-me",
		"secret-store:",
		"file:",
		"vault:",
		"preferences:",
		"Mutually exclusive: choose exactly one",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected template output to contain %q, got %q", snippet, output)
		}
	}
}

func TestPrintTemplateRejectsUnexpectedArguments(t *testing.T) {
	t.Parallel()

	_, err := executeConfigCommand(
		t,
		nil,
		&cliutil.GlobalFlags{},
		"",
		"print-template",
		"unexpected",
	)
	if err == nil {
		t.Fatal("expected print-template to reject positional arguments")
	}
}

func TestAddImportsSingleContextAndSupportsRename(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{},
		`
name: dev
repository:
  filesystem:
    base-dir: /tmp/dev
metadata:
  base-dir: /tmp/meta
`,
		"add",
		"--format", "yaml",
		"--context-name", "dev-imported",
	)
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}

	if len(service.createdContexts) != 1 {
		t.Fatalf("expected one created context, got %d", len(service.createdContexts))
	}
	if got := service.createdContexts[0].Name; got != "dev-imported" {
		t.Fatalf("expected imported context name dev-imported, got %q", got)
	}
	if service.setCurrentName != "" {
		t.Fatalf("set current should not be called, got %q", service.setCurrentName)
	}
}

func TestCreateDefaultsInputFormatToYAML(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{},
		`
name: dev
repository:
  filesystem:
    base-dir: /tmp/dev
`,
		"add",
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	if !service.createCalled {
		t.Fatal("expected create to be called")
	}
	if service.createdContext.Name != "dev" {
		t.Fatalf("expected context name dev, got %q", service.createdContext.Name)
	}
}

func TestCreateInputModeAppliesContextNameFromPositionalArg(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{},
		`
name: from-input
repository:
  filesystem:
    base-dir: /tmp/dev
`,
		"add",
		"from-arg",
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	if !service.createCalled {
		t.Fatal("expected create to be called")
	}
	if service.createdContext.Name != "from-arg" {
		t.Fatalf("expected context name from-arg, got %q", service.createdContext.Name)
	}
}

func TestAddImportsCatalogContexts(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{},
		`
contexts:
  - name: dev
    repository:
      filesystem:
        base-dir: /tmp/dev
  - name: prod
    repository:
      filesystem:
        base-dir: /tmp/prod
current-ctx: prod
`,
		"add",
	)
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}

	if len(service.createdContexts) != 2 {
		t.Fatalf("expected two created contexts, got %d", len(service.createdContexts))
	}
	if service.createdContexts[0].Name != "dev" || service.createdContexts[1].Name != "prod" {
		t.Fatalf("unexpected created contexts: %#v", service.createdContexts)
	}
}

func TestAddSetCurrentForSingleContext(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{},
		`
name: dev
repository:
  filesystem:
    base-dir: /tmp/dev
`,
		"add",
		"--format", "yaml",
		"--context-name", "dev-active",
		"--set-current",
	)
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}

	if len(service.createdContexts) != 1 {
		t.Fatalf("expected one created context, got %d", len(service.createdContexts))
	}
	if got := service.createdContexts[0].Name; got != "dev-active" {
		t.Fatalf("expected imported context name dev-active, got %q", got)
	}
	if service.setCurrentName != "dev-active" {
		t.Fatalf("expected set current dev-active, got %q", service.setCurrentName)
	}
}

func TestAddCatalogContextSelectionAndSetCurrent(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{},
		`
contexts:
  - name: dev
    repository:
      filesystem:
        base-dir: /tmp/dev
  - name: prod
    repository:
      filesystem:
        base-dir: /tmp/prod
current-ctx: prod
`,
		"add",
		"--format", "yaml",
		"--context-name", "prod",
		"--set-current",
	)
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}

	if len(service.createdContexts) != 1 {
		t.Fatalf("expected one created context, got %d", len(service.createdContexts))
	}
	if got := service.createdContexts[0].Name; got != "prod" {
		t.Fatalf("expected imported context prod, got %q", got)
	}
	if service.setCurrentName != "prod" {
		t.Fatalf("expected set current prod, got %q", service.setCurrentName)
	}
}

func TestAddSetCurrentFromCatalogCurrentCtxForMultiImport(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{},
		`
contexts:
  - name: dev
    repository:
      filesystem:
        base-dir: /tmp/dev
  - name: prod
    repository:
      filesystem:
        base-dir: /tmp/prod
current-ctx: prod
`,
		"add",
		"--format", "yaml",
		"--set-current",
	)
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}

	if len(service.createdContexts) != 2 {
		t.Fatalf("expected two created contexts, got %d", len(service.createdContexts))
	}
	if service.setCurrentName != "prod" {
		t.Fatalf("expected set current prod from catalog current-ctx, got %q", service.setCurrentName)
	}
}

func TestAddSetCurrentRequiresResolvableTarget(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{},
		`
contexts:
  - name: dev
    repository:
      filesystem:
        base-dir: /tmp/dev
  - name: prod
    repository:
      filesystem:
        base-dir: /tmp/prod
`,
		"add",
		"--format", "yaml",
		"--set-current",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if service.createCalled {
		t.Fatal("expected create to be skipped when set-current target is ambiguous")
	}
}

func TestAddRejectsUnknownCatalogContextName(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{},
		`
contexts:
  - name: dev
    repository:
      filesystem:
        base-dir: /tmp/dev
`,
		"add",
		"--format", "yaml",
		"--context-name", "prod",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if service.createCalled {
		t.Fatal("expected create to be skipped when selected catalog context is missing")
	}
}

func TestAddRejectsCollisionsBeforeCreate(t *testing.T) {
	t.Parallel()

	t.Run("existing_context_name", func(t *testing.T) {
		t.Parallel()

		service := &testContextService{
			listValue: []configdomain.Context{{Name: "dev"}},
		}
		_, err := executeConfigCommand(
			t,
			service,
			&cliutil.GlobalFlags{},
			`
name: dev
repository:
  filesystem:
    base-dir: /tmp/dev
`,
			"add",
			"--format", "yaml",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if service.createCalled {
			t.Fatal("expected create to be skipped when imported context already exists")
		}
	})

	t.Run("duplicate_names_in_input_catalog", func(t *testing.T) {
		t.Parallel()

		service := &testContextService{}
		_, err := executeConfigCommand(
			t,
			service,
			&cliutil.GlobalFlags{},
			`
contexts:
  - name: dev
    repository:
      filesystem:
        base-dir: /tmp/dev
  - name: dev
    repository:
      filesystem:
        base-dir: /tmp/dev2
`,
			"add",
			"--format", "yaml",
		)
		assertTypedCategory(t, err, faults.ValidationError)
		if service.createCalled {
			t.Fatal("expected create to be skipped when input catalog has duplicate names")
		}
	})
}

func TestResolveParsesOverridesAndRejectsInvalidTokens(t *testing.T) {
	t.Parallel()

	t.Run("valid_overrides_are_forwarded", func(t *testing.T) {
		t.Parallel()

		service := &testContextService{
			resolveValue: configdomain.Context{
				Name: "dev",
				Repository: configdomain.Repository{
					ResourceFormat: configdomain.ResourceFormatJSON,
					Filesystem:     &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
			},
		}
		globalFlags := &cliutil.GlobalFlags{
			Context: "dev",
			Output:  cliutil.OutputText,
		}

		_, err := executeConfigCommand(
			t,
			service,
			globalFlags,
			"",
			"resolve",
			"--set", "metadata.base-dir=/tmp/meta",
			"--set", "metadata.bundle=keycloak-bundle:0.0.1",
			"--set", "metadata.bundle-file=/tmp/keycloak-bundle-0.0.1.tar.gz",
			"--set", "repository.resource-format=yaml",
		)
		if err != nil {
			t.Fatalf("resolve returned error: %v", err)
		}

		if service.resolveSelection.Name != "dev" {
			t.Fatalf("expected selection name dev, got %q", service.resolveSelection.Name)
		}
		if got := service.resolveSelection.Overrides["metadata.base-dir"]; got != "/tmp/meta" {
			t.Fatalf("expected metadata override to be forwarded, got %q", got)
		}
		if got := service.resolveSelection.Overrides["metadata.bundle"]; got != "keycloak-bundle:0.0.1" {
			t.Fatalf("expected metadata bundle override to be forwarded, got %q", got)
		}
		if got := service.resolveSelection.Overrides["metadata.bundle-file"]; got != "/tmp/keycloak-bundle-0.0.1.tar.gz" {
			t.Fatalf("expected metadata bundle-file override to be forwarded, got %q", got)
		}
		if got := service.resolveSelection.Overrides["repository.resource-format"]; got != "yaml" {
			t.Fatalf("expected resource format override to be forwarded, got %q", got)
		}
	})

	t.Run("invalid_override_token_fails_validation", func(t *testing.T) {
		t.Parallel()

		service := &testContextService{}
		_, err := executeConfigCommand(t, service, &cliutil.GlobalFlags{}, "", "resolve", "--set", "missing-equals")
		assertTypedCategory(t, err, faults.ValidationError)
		if service.resolveCalled {
			t.Fatal("expected resolve service call to be skipped on override parse failure")
		}
	})
}

func TestResolveUsesPositionalContextNameWhenProvided(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		resolveValue: configdomain.Context{
			Name: "prod",
			Repository: configdomain.Repository{
				ResourceFormat: configdomain.ResourceFormatYAML,
				Filesystem:     &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
			},
		},
	}

	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{Output: cliutil.OutputText},
		"",
		"resolve",
		"prod",
	)
	if err != nil {
		t.Fatalf("resolve returned error: %v", err)
	}
	if service.resolveSelection.Name != "prod" {
		t.Fatalf("expected resolve to use positional context prod, got %q", service.resolveSelection.Name)
	}
}

func TestResolveRejectsContextNameConflictBetweenPositionalAndFlag(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{Context: "dev"},
		"",
		"resolve",
		"prod",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if service.resolveCalled {
		t.Fatal("expected resolve service call to be skipped on context name conflict")
	}
}

func TestConfigOutputAcrossFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		format          string
		commandArgs     []string
		expectedSnippet string
	}{
		{name: "list_text", format: cliutil.OutputText, commandArgs: []string{"list"}, expectedSnippet: "dev\nprod\n"},
		{name: "list_json", format: cliutil.OutputJSON, commandArgs: []string{"list"}, expectedSnippet: "\"Name\": \"dev\""},
		{name: "list_yaml", format: cliutil.OutputYAML, commandArgs: []string{"list"}, expectedSnippet: "- name: dev"},
		{name: "current_text", format: cliutil.OutputText, commandArgs: []string{"current"}, expectedSnippet: "dev\n"},
		{name: "current_json", format: cliutil.OutputJSON, commandArgs: []string{"current"}, expectedSnippet: "\"Name\": \"dev\""},
		{name: "current_yaml", format: cliutil.OutputYAML, commandArgs: []string{"current"}, expectedSnippet: "name: dev"},
		{name: "resolve_text", format: cliutil.OutputText, commandArgs: []string{"resolve"}, expectedSnippet: "prod\n"},
		{name: "resolve_json", format: cliutil.OutputJSON, commandArgs: []string{"resolve"}, expectedSnippet: "\"Name\": \"prod\""},
		{name: "resolve_yaml", format: cliutil.OutputYAML, commandArgs: []string{"resolve"}, expectedSnippet: "name: prod"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &testContextService{
				listValue: []configdomain.Context{{Name: "dev"}, {Name: "prod"}},
				currentValue: configdomain.Context{
					Name: "dev",
				},
				resolveValue: configdomain.Context{
					Name: "prod",
					Repository: configdomain.Repository{
						ResourceFormat: configdomain.ResourceFormatYAML,
						Filesystem:     &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
					},
				},
			}

			globalFlags := &cliutil.GlobalFlags{
				Context: "prod",
				Output:  tt.format,
			}
			output, err := executeConfigCommand(t, service, globalFlags, "", tt.commandArgs...)
			if err != nil {
				t.Fatalf("command returned error: %v", err)
			}
			if !strings.Contains(output, tt.expectedSnippet) {
				t.Fatalf("expected output to contain %q, got %q", tt.expectedSnippet, output)
			}
		})
	}
}

func TestCheckReportsConfiguredComponents(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metadataDir := filepath.Join(repoDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("failed to create metadata directory: %v", err)
	}

	contextService := &testContextService{
		resolveValue: configdomain.Context{
			Name: "dev",
			Repository: configdomain.Repository{
				Filesystem: &configdomain.FilesystemRepository{BaseDir: repoDir},
			},
			Metadata: configdomain.Metadata{BaseDir: metadataDir},
		},
	}

	deps := cliutil.CommandDependencies{
		Contexts:       contextService,
		ResourceStore:  &testRepositoryService{},
		RepositorySync: &testRepositoryService{},
		Metadata:       &testMetadataService{},
	}
	globalFlags := &cliutil.GlobalFlags{Output: cliutil.OutputText}

	output, err := executeConfigCommandWithDeps(t, deps, globalFlags, "", "check")
	if err != nil {
		t.Fatalf("check returned error: %v", err)
	}

	expectedSnippets := []string{
		`Config check for context "dev"`,
		"[OK] context",
		"[OK] repository",
		"[OK] metadata",
		"[SKIP] managed-server",
		"[SKIP] secret-store",
		"Result: PASS",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected output to contain %q, got %q", snippet, output)
		}
	}
}

func TestCheckReportsMetadataBundleAsAccessible(t *testing.T) {
	t.Parallel()

	contextService := &testContextService{
		resolveValue: configdomain.Context{
			Name: "dev",
			Repository: configdomain.Repository{
				Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
			},
			Metadata: configdomain.Metadata{Bundle: "keycloak-bundle:0.0.1"},
		},
	}

	deps := cliutil.CommandDependencies{
		Contexts:       contextService,
		ResourceStore:  &testRepositoryService{},
		RepositorySync: &testRepositoryService{},
		Metadata:       &testMetadataService{},
	}
	globalFlags := &cliutil.GlobalFlags{Output: cliutil.OutputText}

	output, err := executeConfigCommandWithDeps(t, deps, globalFlags, "", "check")
	if err != nil {
		t.Fatalf("check returned error: %v", err)
	}

	expectedSnippets := []string{
		"[OK] metadata",
		"metadata bundle is accessible",
		"Result: PASS",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected output to contain %q, got %q", snippet, output)
		}
	}
}

func TestCheckReportsMetadataBundleFileAsAccessible(t *testing.T) {
	t.Parallel()

	contextService := &testContextService{
		resolveValue: configdomain.Context{
			Name: "dev",
			Repository: configdomain.Repository{
				Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
			},
			Metadata: configdomain.Metadata{BundleFile: "/tmp/keycloak-bundle-0.0.1.tar.gz"},
		},
	}

	deps := cliutil.CommandDependencies{
		Contexts:       contextService,
		ResourceStore:  &testRepositoryService{},
		RepositorySync: &testRepositoryService{},
		Metadata:       &testMetadataService{},
	}
	globalFlags := &cliutil.GlobalFlags{Output: cliutil.OutputText}

	output, err := executeConfigCommandWithDeps(t, deps, globalFlags, "", "check")
	if err != nil {
		t.Fatalf("check returned error: %v", err)
	}

	expectedSnippets := []string{
		"[OK] metadata",
		"metadata bundle is accessible",
		"Result: PASS",
	}
	for _, snippet := range expectedSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected output to contain %q, got %q", snippet, output)
		}
	}
}

func TestCheckWarnsForReachableManagedServerProbeErrors(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metadataDir := filepath.Join(repoDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("failed to create metadata directory: %v", err)
	}

	contextService := &testContextService{
		resolveValue: configdomain.Context{
			Name: "dev",
			Repository: configdomain.Repository{
				Filesystem: &configdomain.FilesystemRepository{BaseDir: repoDir},
			},
			Metadata: configdomain.Metadata{BaseDir: metadataDir},
			ManagedServer: &configdomain.ManagedServer{
				HTTP: &configdomain.HTTPServer{
					BaseURL: "http://127.0.0.1:8080",
					Auth: &configdomain.HTTPAuth{
						CustomHeaders: []configdomain.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "x"}},
					},
				},
			},
		},
	}

	deps := cliutil.CommandDependencies{
		Contexts:       contextService,
		ResourceStore:  &testRepositoryService{},
		RepositorySync: &testRepositoryService{},
		Metadata:       &testMetadataService{},
		Orchestrator:   &testOrchestratorService{listRemoteErr: faults.NewTypedError(faults.NotFoundError, "probe not found", nil)},
	}
	globalFlags := &cliutil.GlobalFlags{Output: cliutil.OutputText}

	output, err := executeConfigCommandWithDeps(t, deps, globalFlags, "", "check")
	if err != nil {
		t.Fatalf("check returned error: %v", err)
	}
	if !strings.Contains(output, "[WARN] managed-server") {
		t.Fatalf("expected warn status for managed server probe, got %q", output)
	}
	if !strings.Contains(output, "Result: PASS") {
		t.Fatalf("expected pass result when only warnings are present, got %q", output)
	}
}

func TestCheckFailsWhenConfiguredComponentsAreUnavailable(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	metadataDir := filepath.Join(repoDir, "metadata")
	if err := os.MkdirAll(metadataDir, 0o755); err != nil {
		t.Fatalf("failed to create metadata directory: %v", err)
	}

	contextService := &testContextService{
		resolveValue: configdomain.Context{
			Name: "prod",
			Repository: configdomain.Repository{
				Filesystem: &configdomain.FilesystemRepository{BaseDir: repoDir},
			},
			Metadata: configdomain.Metadata{BaseDir: metadataDir},
			ManagedServer: &configdomain.ManagedServer{
				HTTP: &configdomain.HTTPServer{
					BaseURL: "http://127.0.0.1:8080",
					Auth: &configdomain.HTTPAuth{
						CustomHeaders: []configdomain.HeaderTokenAuth{{Header: "Authorization", Prefix: "Bearer", Value: "x"}},
					},
				},
			},
			SecretStore: &configdomain.SecretStore{
				File: &configdomain.FileSecretStore{Path: "/tmp/secrets.json", Passphrase: "pass"},
			},
		},
	}

	deps := cliutil.CommandDependencies{
		Contexts:       contextService,
		ResourceStore:  &testRepositoryService{},
		RepositorySync: &testRepositoryService{},
		Metadata:       &testMetadataService{},
		Orchestrator:   &testOrchestratorService{listRemoteErr: faults.NewTypedError(faults.AuthError, "managed server auth failed", nil)},
		Secrets:        &testSecretProviderService{listErr: faults.NewTypedError(faults.TransportError, "secret store unavailable", nil)},
	}
	globalFlags := &cliutil.GlobalFlags{Output: cliutil.OutputText}

	output, err := executeConfigCommandWithDeps(t, deps, globalFlags, "", "check")
	assertTypedCategory(t, err, faults.ValidationError)

	if !strings.Contains(output, "[FAIL] managed-server") {
		t.Fatalf("expected managed-server failure in output, got %q", output)
	}
	if !strings.Contains(output, "[FAIL] secret-store") {
		t.Fatalf("expected secret-store failure in output, got %q", output)
	}
	if !strings.Contains(output, "Result: FAIL") {
		t.Fatalf("expected fail result in output, got %q", output)
	}
}

func TestCheckUsesPositionalContextNameWhenProvided(t *testing.T) {
	t.Parallel()

	contextService := &testContextService{
		resolveValue: configdomain.Context{
			Name: "prod",
			Repository: configdomain.Repository{
				Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
			},
			Metadata: configdomain.Metadata{Bundle: "keycloak-bundle:0.0.1"},
		},
	}

	deps := cliutil.CommandDependencies{
		Contexts:       contextService,
		RepositorySync: &testRepositoryService{},
		Metadata:       &testMetadataService{},
	}

	_, err := executeConfigCommandWithDeps(t, deps, &cliutil.GlobalFlags{Output: cliutil.OutputText}, "", "check", "prod")
	if err != nil {
		t.Fatalf("check returned error: %v", err)
	}
	if contextService.resolveSelection.Name != "prod" {
		t.Fatalf("expected check to resolve positional context prod, got %q", contextService.resolveSelection.Name)
	}
}

func TestCheckRejectsContextNameConflictBetweenPositionalAndFlag(t *testing.T) {
	t.Parallel()

	contextService := &testContextService{}
	_, err := executeConfigCommandWithDeps(
		t,
		cliutil.CommandDependencies{Contexts: contextService},
		&cliutil.GlobalFlags{Context: "dev"},
		"",
		"check",
		"prod",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if contextService.resolveCalled {
		t.Fatal("expected resolve service call to be skipped on context name conflict")
	}
}

func TestInitInitializesRepositoryAndMetadata(t *testing.T) {
	t.Parallel()

	contextService := &testContextService{
		resolveValue: configdomain.Context{
			Name: "prod",
			Repository: configdomain.Repository{
				Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
			},
			Metadata: configdomain.Metadata{Bundle: "keycloak-bundle:0.0.1"},
		},
	}
	repositoryService := &testRepositoryService{}
	metadataService := &testMetadataService{}

	deps := cliutil.CommandDependencies{
		Contexts:       contextService,
		RepositorySync: repositoryService,
		Metadata:       metadataService,
	}

	_, err := executeConfigCommandWithDeps(
		t,
		deps,
		&cliutil.GlobalFlags{Output: cliutil.OutputText},
		"",
		"init",
		"prod",
	)
	if err != nil {
		t.Fatalf("init returned error: %v", err)
	}
	if contextService.resolveSelection.Name != "prod" {
		t.Fatalf("expected init to resolve positional context prod, got %q", contextService.resolveSelection.Name)
	}
	if !repositoryService.initCalled {
		t.Fatal("expected repository init to be called")
	}
	if len(metadataService.resolvePaths) != 1 || metadataService.resolvePaths[0] != "/" {
		t.Fatalf("expected metadata resolve on root path, got %#v", metadataService.resolvePaths)
	}
}

func TestInitRejectsContextNameConflictBetweenPositionalAndFlag(t *testing.T) {
	t.Parallel()

	contextService := &testContextService{}
	repositoryService := &testRepositoryService{}
	metadataService := &testMetadataService{}

	deps := cliutil.CommandDependencies{
		Contexts:       contextService,
		RepositorySync: repositoryService,
		Metadata:       metadataService,
	}

	_, err := executeConfigCommandWithDeps(
		t,
		deps,
		&cliutil.GlobalFlags{Context: "dev"},
		"",
		"init",
		"prod",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if contextService.resolveCalled {
		t.Fatal("expected resolve service call to be skipped on context name conflict")
	}
	if repositoryService.initCalled {
		t.Fatal("expected repository init to be skipped on context name conflict")
	}
}

func TestCreateInteractivePromptFlow(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"dev", "/tmp/repo", "/tmp/meta", "https://api.example.com", "", "Authorization", "Bearer", "token-dev"},
		selects:     []string{configdomain.ResourceFormatYAML, "filesystem", "custom-headers"},
		confirms:    []bool{false, false, false, false, false, false},
	}

	_, err := executeConfigCommandWithPrompter(
		t,
		service,
		&cliutil.GlobalFlags{},
		prompter,
		"",
		"add",
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if !service.createCalled {
		t.Fatal("expected create to be called")
	}
	if service.createdContext.Name != "dev" {
		t.Fatalf("expected context name dev, got %q", service.createdContext.Name)
	}
	if service.createdContext.Repository.ResourceFormat != configdomain.ResourceFormatYAML {
		t.Fatalf("expected yaml resource format, got %q", service.createdContext.Repository.ResourceFormat)
	}
	if service.createdContext.Repository.Filesystem == nil || service.createdContext.Repository.Filesystem.BaseDir != "/tmp/repo" {
		t.Fatalf("unexpected repository config: %#v", service.createdContext.Repository)
	}
	if service.createdContext.Metadata.BaseDir != "/tmp/meta" {
		t.Fatalf("expected metadata base-dir /tmp/meta, got %q", service.createdContext.Metadata.BaseDir)
	}
	if service.createdContext.ManagedServer == nil || service.createdContext.ManagedServer.HTTP == nil {
		t.Fatal("expected managed-server configuration")
	}
	if len(prompter.selectPrompts) == 0 || prompter.selectPrompts[0] != "Select resource format (optional; remote-default keeps remote resource format)" {
		t.Fatalf("expected optional resource format prompt, got %#v", prompter.selectPrompts)
	}
}

func TestCreateInteractivePromptFlowDefaultsMetadataBaseDirToRepoBaseDir(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"dev", "/tmp/repo", "", "https://api.example.com", "", "Authorization", "Bearer", "token-dev"},
		selects:     []string{configdomain.ResourceFormatYAML, "filesystem", "custom-headers"},
		confirms:    []bool{false, false, false, false, false, false},
	}

	_, err := executeConfigCommandWithPrompter(
		t,
		service,
		&cliutil.GlobalFlags{},
		prompter,
		"",
		"add",
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	if service.createdContext.Metadata.BaseDir != "/tmp/repo" {
		t.Fatalf("expected metadata base-dir to default to repository base-dir /tmp/repo, got %q", service.createdContext.Metadata.BaseDir)
	}
	if service.createdContext.Repository.ResourceFormat != configdomain.ResourceFormatYAML {
		t.Fatalf("expected yaml resource format, got %q", service.createdContext.Repository.ResourceFormat)
	}
	if len(prompter.inputPrompts) < 3 {
		t.Fatalf("expected at least 3 input prompts, got %d", len(prompter.inputPrompts))
	}
	if got := prompter.inputPrompts[2]; got != "Metadata base-dir (defaults to /tmp/repo): " {
		t.Fatalf("expected metadata prompt with repository base-dir value, got %q", got)
	}
}

func TestCreateInteractivePromptFlowSupportsManagedServerProxy(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs: []string{
			"dev",
			"/tmp/repo",
			"/tmp/meta",
			"https://api.example.com",
			"",
			"http://proxy.example.com:3128",
			"",
			"localhost,127.0.0.1",
			"proxy-user",
			"proxy-pass",
			"Authorization",
			"Bearer",
			"token-dev",
		},
		selects: []string{
			configdomain.ResourceFormatYAML,
			"filesystem",
			"custom-headers",
		},
		confirms: []bool{
			false,
			true,
			true,
			false,
			false,
			false,
			false,
		},
	}

	_, err := executeConfigCommandWithPrompter(
		t,
		service,
		&cliutil.GlobalFlags{},
		prompter,
		"",
		"add",
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	if service.createdContext.ManagedServer == nil || service.createdContext.ManagedServer.HTTP == nil {
		t.Fatal("expected managed-server configuration")
	}
	if service.createdContext.ManagedServer.HTTP.Proxy == nil {
		t.Fatal("expected managed-server proxy configuration")
	}

	proxy := service.createdContext.ManagedServer.HTTP.Proxy
	if proxy.HTTPURL != "http://proxy.example.com:3128" {
		t.Fatalf("expected proxy http-url, got %q", proxy.HTTPURL)
	}
	if proxy.HTTPSURL != "" {
		t.Fatalf("expected empty proxy https-url, got %q", proxy.HTTPSURL)
	}
	if proxy.NoProxy != "localhost,127.0.0.1" {
		t.Fatalf("expected proxy no-proxy, got %q", proxy.NoProxy)
	}
	if proxy.Auth == nil {
		t.Fatal("expected proxy auth configuration")
	}
	if proxy.Auth.Username != "proxy-user" || proxy.Auth.Password != "proxy-pass" {
		t.Fatalf("unexpected proxy auth values: %#v", proxy.Auth)
	}
}

func TestCreateInteractivePromptFlowUsesPositionalName(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"/tmp/repo", "/tmp/meta", "https://api.example.com", "", "Authorization", "Bearer", "token-dev"},
		selects:     []string{configdomain.ResourceFormatYAML, "filesystem", "custom-headers"},
		confirms:    []bool{false, false, false, false, false, false},
	}

	_, err := executeConfigCommandWithPrompter(
		t,
		service,
		&cliutil.GlobalFlags{},
		prompter,
		"",
		"add",
		"dev-from-arg",
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	if !service.createCalled {
		t.Fatal("expected create to be called")
	}
	if service.createdContext.Name != "dev-from-arg" {
		t.Fatalf("expected context name dev-from-arg, got %q", service.createdContext.Name)
	}
	if len(prompter.inputPrompts) < 1 {
		t.Fatal("expected input prompts for repository settings")
	}
	if got := prompter.inputPrompts[0]; got != "Repository base-dir: " {
		t.Fatalf("expected first prompt to skip context name and ask repository base-dir, got %q", got)
	}
}

func TestCreateInteractivePromptFlowUsesContextFlagName(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"/tmp/repo", "/tmp/meta", "https://api.example.com", "", "Authorization", "Bearer", "token-dev"},
		selects:     []string{configdomain.ResourceFormatYAML, "filesystem", "custom-headers"},
		confirms:    []bool{false, false, false, false, false, false},
	}

	_, err := executeConfigCommandWithPrompter(
		t,
		service,
		&cliutil.GlobalFlags{Context: "dev-from-flag"},
		prompter,
		"",
		"add",
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	if !service.createCalled {
		t.Fatal("expected create to be called")
	}
	if service.createdContext.Name != "dev-from-flag" {
		t.Fatalf("expected context name dev-from-flag, got %q", service.createdContext.Name)
	}
	if len(prompter.inputPrompts) < 1 {
		t.Fatal("expected input prompts for repository settings")
	}
	if got := prompter.inputPrompts[0]; got != "Repository base-dir: " {
		t.Fatalf("expected first prompt to skip context name and ask repository base-dir, got %q", got)
	}
}

func TestCreateRejectsContextNameConflictBetweenPositionalAndFlag(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	_, err := executeConfigCommand(
		t,
		service,
		&cliutil.GlobalFlags{Context: "dev-flag"},
		"",
		"add",
		"dev-arg",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if service.createCalled {
		t.Fatal("expected create call to be skipped on context name conflict")
	}
}

func TestCreateInteractivePromptFlowAllowsRemoteDefaultResourceFormat(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"dev", "/tmp/repo", "/tmp/meta", "https://api.example.com", "", "Authorization", "Bearer", "token-dev"},
		selects:     []string{resourceFormatRemoteDefaultOption, "filesystem", "custom-headers"},
		confirms:    []bool{false, false, false, false, false, false},
	}

	_, err := executeConfigCommandWithPrompter(
		t,
		service,
		&cliutil.GlobalFlags{},
		prompter,
		"",
		"add",
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}
	if service.createdContext.Repository.ResourceFormat != "" {
		t.Fatalf("expected empty resource format for remote-default selection, got %q", service.createdContext.Repository.ResourceFormat)
	}
}

func TestCreateInteractivePromptFlowGitLocalAutoInitCanBeDisabled(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"dev", "/tmp/repo-git", "/tmp/meta", "https://api.example.com", "", "Authorization", "Bearer", "token-dev"},
		selects:     []string{configdomain.ResourceFormatYAML, "git", "custom-headers"},
		confirms: []bool{
			false,
			false,
			false,
			false,
			false,
			false,
			false,
			false,
		},
	}

	_, err := executeConfigCommandWithPrompter(
		t,
		service,
		&cliutil.GlobalFlags{},
		prompter,
		"",
		"add",
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	if service.createdContext.Repository.Git == nil {
		t.Fatalf("expected git repository config, got %#v", service.createdContext.Repository)
	}
	if service.createdContext.Repository.Git.Local.AutoInitEnabled() {
		t.Fatal("expected git local auto-init to be disabled")
	}
	if service.createdContext.Repository.Git.Local.AutoInit == nil {
		t.Fatal("expected auto-init=false to be persisted explicitly")
	}
}

func TestCreateInteractivePromptFlowSupportsOptionalSectionsAndOneOfBranches(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs: []string{
			"/tmp/repo",
			"",
			"https://api.example.com",
			"https://api.example.com/openapi.yaml",
			"X-Tenant",
			"acme",
			"",
			"https://idp.example.com/token",
			"",
			"client-id",
			"client-secret",
			"",
			"",
			"scope-a",
			"",
			"/tmp/secrets.json",
			"/tmp/key.txt",
			"1",
			"65536",
			"4",
			"env",
			"dev",
			"",
		},
		selects: []string{
			configdomain.ResourceFormatJSON,
			"filesystem",
			"oauth2",
			"file",
			"key-file",
		},
		confirms: []bool{
			true,
			false,
			false,
			true,
			true,
			true,
		},
	}

	_, err := executeConfigCommandWithPrompter(
		t,
		service,
		&cliutil.GlobalFlags{},
		prompter,
		"",
		"add",
		"full-context",
	)
	if err != nil {
		t.Fatalf("create returned error: %v", err)
	}

	if !service.createCalled {
		t.Fatal("expected create to be called")
	}

	if service.createdContext.Name != "full-context" {
		t.Fatalf("expected context name full-context, got %q", service.createdContext.Name)
	}
	if service.createdContext.Repository.ResourceFormat != configdomain.ResourceFormatJSON {
		t.Fatalf("expected repository format json, got %q", service.createdContext.Repository.ResourceFormat)
	}
	if service.createdContext.ManagedServer == nil || service.createdContext.ManagedServer.HTTP == nil {
		t.Fatal("expected managed-server http configuration")
	}
	if service.createdContext.ManagedServer.HTTP.Auth == nil {
		t.Fatal("expected managed-server auth configuration")
	}
	if service.createdContext.ManagedServer.HTTP.Auth.OAuth2 == nil {
		t.Fatal("expected managed-server oauth2 configuration")
	}
	if service.createdContext.ManagedServer.HTTP.Auth.BasicAuth != nil {
		t.Fatal("basic auth should not be configured when oauth2 is selected")
	}
	if len(service.createdContext.ManagedServer.HTTP.Auth.CustomHeaders) != 0 {
		t.Fatal("custom-headers auth should not be configured when oauth2 is selected")
	}
	if service.createdContext.ManagedServer.HTTP.Auth.OAuth2.GrantType != configdomain.OAuthClientCreds {
		t.Fatalf(
			"expected oauth2 grant-type default %q, got %q",
			configdomain.OAuthClientCreds,
			service.createdContext.ManagedServer.HTTP.Auth.OAuth2.GrantType,
		)
	}

	if service.createdContext.SecretStore == nil || service.createdContext.SecretStore.File == nil {
		t.Fatal("expected file secret-store configuration")
	}
	if service.createdContext.SecretStore.File.KeyFile != "/tmp/key.txt" {
		t.Fatalf("expected secret-store key-file /tmp/key.txt, got %q", service.createdContext.SecretStore.File.KeyFile)
	}
	if service.createdContext.SecretStore.File.Key != "" {
		t.Fatal("secret-store key should not be set when key-file source is selected")
	}
	if service.createdContext.SecretStore.File.Passphrase != "" || service.createdContext.SecretStore.File.PassphraseFile != "" {
		t.Fatal("secret-store passphrase fields should not be set when key-file source is selected")
	}
	if service.createdContext.SecretStore.File.KDF == nil {
		t.Fatal("expected secret-store KDF configuration")
	}
	if service.createdContext.SecretStore.File.KDF.Time != 1 ||
		service.createdContext.SecretStore.File.KDF.Memory != 65536 ||
		service.createdContext.SecretStore.File.KDF.Threads != 4 {
		t.Fatalf("unexpected KDF values: %#v", service.createdContext.SecretStore.File.KDF)
	}

	if value := service.createdContext.Preferences["env"]; value != "dev" {
		t.Fatalf("expected preference env=dev, got %q", value)
	}
	if len(prompter.inputPrompts) == 0 || prompter.inputPrompts[0] != "Repository base-dir: " {
		t.Fatalf("expected first prompt to skip context name and ask repository base-dir, got %q", prompter.inputPrompts)
	}
}

func TestUseInteractiveSelection(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{{Name: "dev"}, {Name: "prod"}},
	}
	prompter := &mockPrompter{
		interactive: true,
		selects:     []string{"prod"},
	}

	_, err := executeConfigCommandWithPrompter(t, service, &cliutil.GlobalFlags{}, prompter, "", "use")
	if err != nil {
		t.Fatalf("use returned error: %v", err)
	}
	if service.setCurrentName != "prod" {
		t.Fatalf("expected set current prod, got %q", service.setCurrentName)
	}
}

func TestShowUsesContextFlagWhenProvided(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{{
			Name: "prod",
			Repository: configdomain.Repository{
				ResourceFormat: configdomain.ResourceFormatYAML,
				Filesystem:     &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
			},
		}},
	}
	prompter := &mockPrompter{interactive: true}
	globalFlags := &cliutil.GlobalFlags{
		Context: "prod",
		Output:  cliutil.OutputText,
	}

	output, err := executeConfigCommandWithPrompter(t, service, globalFlags, prompter, "", "show")
	if err != nil {
		t.Fatalf("show returned error: %v", err)
	}
	if service.resolveCalled {
		t.Fatal("expected show to read stored context without calling resolve")
	}
	if !strings.Contains(output, "name: prod") {
		t.Fatalf("expected YAML output with context name prod, got %q", output)
	}
	if !strings.Contains(output, "resource-format: yaml") {
		t.Fatalf("expected YAML output for full context config, got %q", output)
	}
}

func TestShowUsesPositionalContextNameWhenProvided(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{{
			Name: "prod",
			Repository: configdomain.Repository{
				ResourceFormat: configdomain.ResourceFormatYAML,
				Filesystem:     &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
			},
		}},
	}
	prompter := &mockPrompter{interactive: false}

	output, err := executeConfigCommandWithPrompter(
		t,
		service,
		&cliutil.GlobalFlags{Output: cliutil.OutputText},
		prompter,
		"",
		"show",
		"prod",
	)
	if err != nil {
		t.Fatalf("show returned error: %v", err)
	}
	if service.resolveCalled {
		t.Fatal("expected show to read stored context without calling resolve")
	}
	if !strings.Contains(output, "name: prod") {
		t.Fatalf("expected YAML output with context name prod, got %q", output)
	}
}

func TestShowRejectsContextNameConflictBetweenPositionalAndFlag(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{interactive: true}

	_, err := executeConfigCommandWithPrompter(
		t,
		service,
		&cliutil.GlobalFlags{Context: "dev", Output: cliutil.OutputText},
		prompter,
		"",
		"show",
		"prod",
	)
	assertTypedCategory(t, err, faults.ValidationError)
	if service.resolveCalled {
		t.Fatal("expected show resolve call to be skipped on context name conflict")
	}
}

func TestShowInteractiveSelectionWhenContextFlagMissing(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{
			{
				Name: "dev",
				Repository: configdomain.Repository{
					ResourceFormat: configdomain.ResourceFormatJSON,
					Filesystem:     &configdomain.FilesystemRepository{BaseDir: "/tmp/dev"},
				},
			},
			{
				Name: "prod",
				Repository: configdomain.Repository{
					ResourceFormat: configdomain.ResourceFormatYAML,
					Filesystem:     &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
				},
			},
		},
	}
	prompter := &mockPrompter{
		interactive: true,
		selects:     []string{"dev"},
	}
	globalFlags := &cliutil.GlobalFlags{Output: cliutil.OutputText}

	output, err := executeConfigCommandWithPrompter(t, service, globalFlags, prompter, "", "show")
	if err != nil {
		t.Fatalf("show returned error: %v", err)
	}
	if service.resolveCalled {
		t.Fatal("expected interactive show to read stored context without calling resolve")
	}
	if !strings.Contains(output, "name: dev") {
		t.Fatalf("expected YAML output with context name dev, got %q", output)
	}
	if !strings.Contains(output, "resource-format: json") {
		t.Fatalf("expected YAML output for full context config, got %q", output)
	}
}

func TestShowOmitsDefaultMetadataBaseDir(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{
			{
				Name: "dev",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
				},
				ManagedServer: &configdomain.ManagedServer{
					HTTP: &configdomain.HTTPServer{
						BaseURL: "https://example.com/api",
						Auth: &configdomain.HTTPAuth{
							CustomHeaders: []configdomain.HeaderTokenAuth{{
								Header: "Authorization",
								Value:  "token",
							}},
						},
					},
				},
				Metadata: configdomain.Metadata{BaseDir: "/tmp/repo"},
			},
		},
	}

	output, err := executeConfigCommandWithPrompter(
		t,
		service,
		&cliutil.GlobalFlags{Context: "dev", Output: cliutil.OutputText},
		&mockPrompter{interactive: false},
		"",
		"show",
	)
	if err != nil {
		t.Fatalf("show returned error: %v", err)
	}
	if strings.Contains(output, "metadata:") {
		t.Fatalf("expected compact show output to omit default metadata block, got %q", output)
	}
}

func TestShowRequiresContextInNonInteractiveModeWhenFlagMissing(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{{Name: "dev"}},
	}
	prompter := &mockPrompter{interactive: false}
	globalFlags := &cliutil.GlobalFlags{Output: cliutil.OutputText}

	_, err := executeConfigCommandWithPrompter(t, service, globalFlags, prompter, "", "show")
	assertTypedCategory(t, err, faults.ValidationError)
}

func TestRenameInteractiveSelectionAndInput(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{{Name: "dev"}, {Name: "prod"}},
	}
	prompter := &mockPrompter{
		interactive: true,
		selects:     []string{"dev"},
		inputs:      []string{"development"},
	}

	_, err := executeConfigCommandWithPrompter(t, service, &cliutil.GlobalFlags{}, prompter, "", "rename")
	if err != nil {
		t.Fatalf("rename returned error: %v", err)
	}
	if service.renameFrom != "dev" || service.renameTo != "development" {
		t.Fatalf("unexpected rename call: %q -> %q", service.renameFrom, service.renameTo)
	}
}

func TestDeleteInteractiveSelectionAndConfirm(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{{Name: "dev"}, {Name: "prod"}},
	}
	prompter := &mockPrompter{
		interactive: true,
		selects:     []string{"prod"},
		confirms:    []bool{true},
	}

	_, err := executeConfigCommandWithPrompter(t, service, &cliutil.GlobalFlags{}, prompter, "", "delete")
	if err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if service.deletedName != "prod" {
		t.Fatalf("expected delete prod, got %q", service.deletedName)
	}
}

func TestDeleteInteractiveCanceled(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{{Name: "dev"}},
	}
	prompter := &mockPrompter{
		interactive: true,
		selects:     []string{"dev"},
		confirms:    []bool{false},
	}

	output, err := executeConfigCommandWithPrompter(t, service, &cliutil.GlobalFlags{}, prompter, "", "delete")
	if err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if !strings.Contains(output, "delete canceled") {
		t.Fatalf("expected cancel output, got %q", output)
	}
	if service.deletedName != "" {
		t.Fatalf("delete should not have been called, got %q", service.deletedName)
	}
}

func TestInteractiveCommandsRequireNameInNonInteractiveMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "use_without_name", args: []string{"use"}},
		{name: "rename_without_name", args: []string{"rename"}},
		{name: "delete_without_name", args: []string{"delete"}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := &testContextService{
				listValue: []configdomain.Context{{Name: "dev"}},
			}
			prompter := &mockPrompter{interactive: false}

			_, err := executeConfigCommandWithPrompter(t, service, &cliutil.GlobalFlags{}, prompter, "", tt.args...)
			assertTypedCategory(t, err, faults.ValidationError)
		})
	}
}

func TestUseRenameDeleteWithArgsBypassInteractive(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{interactive: false}

	if _, err := executeConfigCommandWithPrompter(t, service, &cliutil.GlobalFlags{}, prompter, "", "use", "prod"); err != nil {
		t.Fatalf("use returned error: %v", err)
	}
	if service.setCurrentName != "prod" {
		t.Fatalf("expected set current prod, got %q", service.setCurrentName)
	}

	if _, err := executeConfigCommandWithPrompter(t, service, &cliutil.GlobalFlags{}, prompter, "", "rename", "dev", "development"); err != nil {
		t.Fatalf("rename returned error: %v", err)
	}
	if service.renameFrom != "dev" || service.renameTo != "development" {
		t.Fatalf("unexpected rename call: %q -> %q", service.renameFrom, service.renameTo)
	}

	if _, err := executeConfigCommandWithPrompter(t, service, &cliutil.GlobalFlags{}, prompter, "", "delete", "legacy"); err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if service.deletedName != "legacy" {
		t.Fatalf("expected delete legacy, got %q", service.deletedName)
	}
}

func executeConfigCommand(
	t *testing.T,
	contexts configdomain.ContextService,
	globalFlags *cliutil.GlobalFlags,
	stdin string,
	args ...string,
) (string, error) {
	t.Helper()

	return executeConfigCommandWithDeps(
		t,
		cliutil.CommandDependencies{Contexts: contexts},
		globalFlags,
		stdin,
		args...,
	)
}

func executeConfigCommandWithDeps(
	t *testing.T,
	deps cliutil.CommandDependencies,
	globalFlags *cliutil.GlobalFlags,
	stdin string,
	args ...string,
) (string, error) {
	t.Helper()

	command := NewCommand(deps, globalFlags)
	output := &bytes.Buffer{}
	command.SetOut(output)
	command.SetErr(io.Discard)
	command.SetIn(strings.NewReader(stdin))
	command.SetArgs(args)

	err := command.Execute()
	return output.String(), err
}

func executeConfigCommandWithPrompter(
	t *testing.T,
	contexts configdomain.ContextService,
	globalFlags *cliutil.GlobalFlags,
	prompter configPrompter,
	stdin string,
	args ...string,
) (string, error) {
	t.Helper()

	return executeConfigCommandWithDepsAndPrompter(
		t,
		cliutil.CommandDependencies{Contexts: contexts},
		globalFlags,
		prompter,
		stdin,
		args...,
	)
}

func executeConfigCommandWithDepsAndPrompter(
	t *testing.T,
	deps cliutil.CommandDependencies,
	globalFlags *cliutil.GlobalFlags,
	prompter configPrompter,
	stdin string,
	args ...string,
) (string, error) {
	t.Helper()

	command := newCommandWithPrompter(deps, globalFlags, prompter)
	output := &bytes.Buffer{}
	command.SetOut(output)
	command.SetErr(io.Discard)
	command.SetIn(strings.NewReader(stdin))
	command.SetArgs(args)

	err := command.Execute()
	return output.String(), err
}

type testContextService struct {
	listValue        []configdomain.Context
	currentValue     configdomain.Context
	resolveValue     configdomain.Context
	resolveSelection configdomain.ContextSelection

	createdContext  configdomain.Context
	createdContexts []configdomain.Context
	setCurrentName  string
	deletedName     string
	renameFrom      string
	renameTo        string

	createCalled   bool
	updateCalled   bool
	validateCalled bool
	resolveCalled  bool
}

func (s *testContextService) Create(_ context.Context, cfg configdomain.Context) error {
	s.createCalled = true
	s.createdContext = cfg
	s.createdContexts = append(s.createdContexts, cfg)
	return nil
}

func (s *testContextService) Update(context.Context, configdomain.Context) error {
	s.updateCalled = true
	return nil
}

func (s *testContextService) Delete(_ context.Context, name string) error {
	s.deletedName = name
	return nil
}

func (s *testContextService) Rename(_ context.Context, from string, to string) error {
	s.renameFrom = from
	s.renameTo = to
	return nil
}

func (s *testContextService) List(context.Context) ([]configdomain.Context, error) {
	return s.listValue, nil
}

func (s *testContextService) SetCurrent(_ context.Context, name string) error {
	s.setCurrentName = name
	return nil
}

func (s *testContextService) GetCurrent(context.Context) (configdomain.Context, error) {
	return s.currentValue, nil
}

func (s *testContextService) ResolveContext(_ context.Context, selection configdomain.ContextSelection) (configdomain.Context, error) {
	s.resolveCalled = true
	s.resolveSelection = selection
	return s.resolveValue, nil
}

func (s *testContextService) Validate(context.Context, configdomain.Context) error {
	s.validateCalled = true
	return nil
}

type testRepositoryService struct {
	initCalled    bool
	initErr       error
	checkErr      error
	syncStatusErr error
	syncStatus    repository.SyncReport
}

func (s *testRepositoryService) Save(context.Context, string, resource.Value) error { return nil }
func (s *testRepositoryService) Get(context.Context, string) (resource.Value, error) {
	return map[string]any{}, nil
}
func (s *testRepositoryService) Delete(context.Context, string, repository.DeletePolicy) error {
	return nil
}
func (s *testRepositoryService) List(context.Context, string, repository.ListPolicy) ([]resource.Resource, error) {
	return nil, nil
}
func (s *testRepositoryService) Exists(context.Context, string) (bool, error) { return false, nil }
func (s *testRepositoryService) Move(context.Context, string, string) error   { return nil }
func (s *testRepositoryService) Init(context.Context) error {
	s.initCalled = true
	return s.initErr
}
func (s *testRepositoryService) Refresh(context.Context) error { return nil }
func (s *testRepositoryService) Clean(context.Context) error   { return nil }
func (s *testRepositoryService) Reset(context.Context, repository.ResetPolicy) error {
	return nil
}
func (s *testRepositoryService) Check(context.Context) error { return s.checkErr }
func (s *testRepositoryService) Push(context.Context, repository.PushPolicy) error {
	return nil
}
func (s *testRepositoryService) SyncStatus(context.Context) (repository.SyncReport, error) {
	if s.syncStatusErr != nil {
		return repository.SyncReport{}, s.syncStatusErr
	}
	return s.syncStatus, nil
}

type testMetadataService struct {
	resolveErr   error
	resolvePaths []string
}

func (s *testMetadataService) Get(context.Context, string) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}
func (s *testMetadataService) Set(context.Context, string, metadatadomain.ResourceMetadata) error {
	return nil
}
func (s *testMetadataService) Unset(context.Context, string) error { return nil }
func (s *testMetadataService) ResolveForPath(_ context.Context, logicalPath string) (metadatadomain.ResourceMetadata, error) {
	s.resolvePaths = append(s.resolvePaths, logicalPath)
	if s.resolveErr != nil {
		return metadatadomain.ResourceMetadata{}, s.resolveErr
	}
	return metadatadomain.ResourceMetadata{}, nil
}
func (s *testMetadataService) RenderOperationSpec(
	context.Context,
	string,
	metadatadomain.Operation,
	any,
) (metadatadomain.OperationSpec, error) {
	return metadatadomain.OperationSpec{}, nil
}
func (s *testMetadataService) Infer(
	context.Context,
	string,
	metadatadomain.InferenceRequest,
) (metadatadomain.ResourceMetadata, error) {
	return metadatadomain.ResourceMetadata{}, nil
}

type testOrchestratorService struct {
	listRemoteErr error
}

func (s *testOrchestratorService) GetLocal(context.Context, string) (resource.Value, error) {
	return nil, nil
}
func (s *testOrchestratorService) GetRemote(context.Context, string) (resource.Value, error) {
	return nil, nil
}
func (s *testOrchestratorService) Request(context.Context, string, string, resource.Value) (resource.Value, error) {
	return nil, nil
}
func (s *testOrchestratorService) GetOpenAPISpec(context.Context) (resource.Value, error) {
	return nil, nil
}
func (s *testOrchestratorService) Save(context.Context, string, resource.Value) error {
	return nil
}
func (s *testOrchestratorService) Apply(context.Context, string, orchestratordomain.ApplyPolicy) (resource.Resource, error) {
	return resource.Resource{}, nil
}
func (s *testOrchestratorService) ApplyWithValue(context.Context, string, resource.Value, orchestratordomain.ApplyPolicy) (resource.Resource, error) {
	return resource.Resource{}, nil
}
func (s *testOrchestratorService) Create(context.Context, string, resource.Value) (resource.Resource, error) {
	return resource.Resource{}, nil
}
func (s *testOrchestratorService) Update(context.Context, string, resource.Value) (resource.Resource, error) {
	return resource.Resource{}, nil
}
func (s *testOrchestratorService) Delete(context.Context, string, orchestratordomain.DeletePolicy) error {
	return nil
}
func (s *testOrchestratorService) ListLocal(context.Context, string, orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	return nil, nil
}
func (s *testOrchestratorService) ListRemote(context.Context, string, orchestratordomain.ListPolicy) ([]resource.Resource, error) {
	return nil, s.listRemoteErr
}
func (s *testOrchestratorService) Explain(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, nil
}
func (s *testOrchestratorService) Diff(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, nil
}
func (s *testOrchestratorService) Template(context.Context, string, resource.Value) (resource.Value, error) {
	return nil, nil
}

type testSecretProviderService struct {
	listErr error
	keys    []string
}

func (s *testSecretProviderService) Init(context.Context) error { return nil }
func (s *testSecretProviderService) Store(context.Context, string, string) error {
	return nil
}
func (s *testSecretProviderService) Get(context.Context, string) (string, error) {
	return "", nil
}
func (s *testSecretProviderService) Delete(context.Context, string) error { return nil }
func (s *testSecretProviderService) List(context.Context) ([]string, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.keys, nil
}
func (s *testSecretProviderService) MaskPayload(context.Context, resource.Value) (resource.Value, error) {
	return nil, nil
}
func (s *testSecretProviderService) ResolvePayload(context.Context, resource.Value) (resource.Value, error) {
	return nil, nil
}
func (s *testSecretProviderService) NormalizeSecretPlaceholders(context.Context, resource.Value) (resource.Value, error) {
	return nil, nil
}
func (s *testSecretProviderService) DetectSecretCandidates(context.Context, resource.Value) ([]string, error) {
	return nil, nil
}

func assertTypedCategory(t *testing.T, err error, category faults.ErrorCategory) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected %q error, got nil", category)
	}

	var typedErr *faults.TypedError
	if !errors.As(err, &typedErr) {
		t.Fatalf("expected typed error, got %T", err)
	}
	if typedErr.Category != category {
		t.Fatalf("expected %q category, got %q", category, typedErr.Category)
	}
}

type mockPrompter struct {
	interactive   bool
	inputs        []string
	selects       []string
	confirms      []bool
	inputPrompts  []string
	selectPrompts []string
}

func (m *mockPrompter) IsInteractive(*cobra.Command) bool {
	return m.interactive
}

func (m *mockPrompter) Input(_ *cobra.Command, prompt string, _ bool) (string, error) {
	m.inputPrompts = append(m.inputPrompts, prompt)
	if len(m.inputs) == 0 {
		return "", errors.New("missing mock input value")
	}
	value := m.inputs[0]
	m.inputs = m.inputs[1:]
	return value, nil
}

func (m *mockPrompter) Select(_ *cobra.Command, prompt string, _ []string) (string, error) {
	m.selectPrompts = append(m.selectPrompts, prompt)
	if len(m.selects) == 0 {
		return "", errors.New("missing mock select value")
	}
	value := m.selects[0]
	m.selects = m.selects[1:]
	return value, nil
}

func (m *mockPrompter) Confirm(*cobra.Command, string, bool) (bool, error) {
	if len(m.confirms) == 0 {
		return false, errors.New("missing mock confirm value")
	}
	value := m.confirms[0]
	m.confirms = m.confirms[1:]
	return value, nil
}
