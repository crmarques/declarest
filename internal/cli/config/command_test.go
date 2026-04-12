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
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/cliutil"
	managedservicedomain "github.com/crmarques/declarest/managedservice"
	metadatadomain "github.com/crmarques/declarest/metadata"
	orchestratordomain "github.com/crmarques/declarest/orchestrator"
	"github.com/crmarques/declarest/repository"
	"github.com/crmarques/declarest/resource"
	secretsdomain "github.com/crmarques/declarest/secrets"
	"github.com/spf13/cobra"
)

func TestCreateUpdateValidateRejectUnknownFields(t *testing.T) {
	t.Parallel()

	t.Run("create_rejects_unknown_json_field", func(t *testing.T) {
		t.Parallel()

		service := &testContextService{}
		_, err := executeConfigCommand(t, service, &cliutil.GlobalFlags{}, `{
  "name": "dev",
  "repository": {"filesystem": {"baseDir": "/tmp/repo"}},
  "unknown": true
}`, "add", "--content-type", "json")
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
    baseDir: /tmp/repo
unknown: true
`, "update", "--content-type", "yaml")
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
  "repository": {"filesystem": {"baseDir": "/tmp/repo", "extra": true}}
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
		"credentials:",
		"contexts:",
		"currentContext:",
		"repository:",
		"git:",
		"filesystem:",
		"managedService:",
		"healthCheck:",
		"auth:",
		"proxy:",
		"http:",
		"https:",
		"noProxy:",
		"oauth2:",
		"basic:",
		"credentialsRef:",
		"prompt:",
		"persistInSession:",
		"customHeaders:",
		"prefix: Bearer",
		"value: change-me",
		"secretStore:",
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

func TestMigrateRewritesCatalogUsingEditorService(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		catalogValue: configdomain.ContextCatalog{
			CurrentContext: "dev",
			Contexts: []configdomain.Context{
				{
					Name: "dev",
					Repository: configdomain.Repository{
						Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
					},
				},
			},
		},
	}

	output, err := executeConfigCommand(t, service, &cliutil.GlobalFlags{}, "", "migrate")
	if err != nil {
		t.Fatalf("migrate returned error: %v", err)
	}
	if !service.replaceCatalogCalled {
		t.Fatal("expected migrate to rewrite the catalog")
	}
	if service.replacedCatalog.CurrentContext != "dev" {
		t.Fatalf("expected replaced catalog current context dev, got %q", service.replacedCatalog.CurrentContext)
	}
	if strings.TrimSpace(output) != "context catalog migrated" {
		t.Fatalf("expected migrate output, got %q", output)
	}
}

func TestCleanCredentialsInSessionRequiresTargetFlag(t *testing.T) {
	t.Parallel()

	_, err := executeConfigCommand(t, nil, &cliutil.GlobalFlags{}, "", "clean")
	assertTypedCategory(t, err, faults.ValidationError)
	if err == nil || !strings.Contains(err.Error(), "clean target flag") {
		t.Fatalf("expected clean target validation error, got %v", err)
	}
}

func TestCleanCredentialsInSessionRemovesPromptCredentialCacheFiles(t *testing.T) {
	runtimeDir := t.TempDir()
	homeDir := t.TempDir()
	sessionID := "shell-session"
	runtimePath := filepath.Join(runtimeDir, "declarest", "prompt-auth", promptAuthSessionFileName(sessionID))
	legacyPath := filepath.Join(
		homeDir,
		".declarest",
		"sessions",
		promptAuthSessionFileName("DECLAREST_PROMPT_AUTH_SESSION_ID:"+sessionID),
	)

	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	t.Setenv("HOME", homeDir)
	t.Setenv("DECLAREST_PROMPT_AUTH_SESSION_ID", sessionID)
	writePromptAuthCacheFile(t, runtimePath)
	writePromptAuthCacheFile(t, legacyPath)

	output, err := executeConfigCommand(
		t,
		nil,
		&cliutil.GlobalFlags{},
		"",
		"clean",
		"--credentials-in-session",
	)
	if err != nil {
		t.Fatalf("clean returned error: %v", err)
	}
	if strings.TrimSpace(output) != "removed 2 prompt credential session cache files" {
		t.Fatalf("expected clean output, got %q", output)
	}
	if _, err := os.Stat(runtimePath); !os.IsNotExist(err) {
		t.Fatalf("expected runtime cache file to be removed, got err=%v", err)
	}
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("expected legacy cache file to be removed, got err=%v", err)
	}
}

func TestCleanCredentialsInSessionSucceedsWithoutCachedValues(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DECLAREST_PROMPT_AUTH_SESSION_ID", "shell-session")

	output, err := executeConfigCommand(
		t,
		nil,
		&cliutil.GlobalFlags{},
		"",
		"clean",
		"--credentials-in-session",
	)
	if err != nil {
		t.Fatalf("clean returned error: %v", err)
	}
	if strings.TrimSpace(output) != "removed 0 prompt credential session cache files" {
		t.Fatalf("expected clean output, got %q", output)
	}
}

func TestSessionHookCommandOutputsShellCode(t *testing.T) {
	t.Run("bash", func(t *testing.T) {
		output, err := executeConfigCommand(t, nil, &cliutil.GlobalFlags{}, "", "session-hook", "bash")
		if err != nil {
			t.Fatalf("session-hook bash returned error: %v", err)
		}
		required := []string{
			`export DECLAREST_PROMPT_AUTH_SESSION_ID="bash:${BASHPID:-$$}"`,
			"command declarest context clean --credentials-in-session >/dev/null 2>&1 || true",
			`trap 'declarest_prompt_auth_on_exit' EXIT`,
		}
		for _, snippet := range required {
			if !strings.Contains(output, snippet) {
				t.Fatalf("expected bash hook output to contain %q, got %q", snippet, output)
			}
		}
	})

	t.Run("zsh", func(t *testing.T) {
		output, err := executeConfigCommand(t, nil, &cliutil.GlobalFlags{}, "", "session-hook", "zsh")
		if err != nil {
			t.Fatalf("session-hook zsh returned error: %v", err)
		}
		required := []string{
			`export DECLAREST_PROMPT_AUTH_SESSION_ID="zsh:$$"`,
			"command declarest context clean --credentials-in-session >/dev/null 2>&1 || true",
			"typeset -ga zshexit_functions",
		}
		for _, snippet := range required {
			if !strings.Contains(output, snippet) {
				t.Fatalf("expected zsh hook output to contain %q, got %q", snippet, output)
			}
		}
	})
}

func TestSessionHookCommandRejectsUnsupportedShell(t *testing.T) {
	_, err := executeConfigCommand(t, nil, &cliutil.GlobalFlags{}, "", "session-hook", "fish")
	assertTypedCategory(t, err, faults.ValidationError)
	if err == nil || !strings.Contains(err.Error(), "bash or zsh") {
		t.Fatalf("expected shell validation error, got %v", err)
	}
}

func TestResolveManagedServiceHealthCheckProbePathDefaultsToBaseURLPath(t *testing.T) {
	t.Parallel()

	probePath, err := resolveManagedServiceHealthCheckProbePath(configdomain.Context{
		ManagedService: &configdomain.ManagedService{
			HTTP: &configdomain.HTTPServer{
				BaseURL: "https://api.example.invalid/admin/api/45",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected url fallback to succeed, got %v", err)
	}
	if probePath != "/admin/api/45" {
		t.Fatalf("expected probe path /admin/api/45, got %q", probePath)
	}
	if got := renderManagedServiceHealthCheckTarget(configdomain.Context{
		ManagedService: &configdomain.ManagedService{
			HTTP: &configdomain.HTTPServer{
				BaseURL: "https://api.example.invalid/admin/api/45",
			},
		},
	}); got != "https://api.example.invalid/admin/api/45" {
		t.Fatalf("expected rendered target to use url fallback, got %q", got)
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
    baseDir: /tmp/dev
metadata:
  baseDir: /tmp/meta
`,
		"add",
		"--content-type", "yaml",
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
    baseDir: /tmp/dev
`,
		"add",
		"--content-type", "yaml",
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
    baseDir: /tmp/dev
`,
		"add",
		"--content-type", "yaml",
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
        baseDir: /tmp/dev
  - name: prod
    repository:
      filesystem:
        baseDir: /tmp/prod
currentContext: prod
`,
		"add",
		"--content-type", "yaml",
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
    baseDir: /tmp/dev
`,
		"add",
		"--content-type", "yaml",
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
        baseDir: /tmp/dev
  - name: prod
    repository:
      filesystem:
        baseDir: /tmp/prod
currentContext: prod
`,
		"add",
		"--content-type", "yaml",
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

func TestAddSetCurrentFromCatalogCurrentContextForMultiImport(t *testing.T) {
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
        baseDir: /tmp/dev
  - name: prod
    repository:
      filesystem:
        baseDir: /tmp/prod
currentContext: prod
`,
		"add",
		"--content-type", "yaml",
		"--set-current",
	)
	if err != nil {
		t.Fatalf("add returned error: %v", err)
	}

	if len(service.createdContexts) != 2 {
		t.Fatalf("expected two created contexts, got %d", len(service.createdContexts))
	}
	if service.setCurrentName != "prod" {
		t.Fatalf("expected set current prod from catalog currentContext, got %q", service.setCurrentName)
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
        baseDir: /tmp/dev
  - name: prod
    repository:
      filesystem:
        baseDir: /tmp/prod
`,
		"add",
		"--content-type", "yaml",
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
        baseDir: /tmp/dev
`,
		"add",
		"--content-type", "yaml",
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
    baseDir: /tmp/dev
`,
			"add",
			"--content-type", "yaml",
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
        baseDir: /tmp/dev
  - name: dev
    repository:
      filesystem:
        baseDir: /tmp/dev2
`,
			"add",
			"--content-type", "yaml",
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
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
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
			"--set", "metadata.baseDir=/tmp/meta",
			"--set", "metadata.bundle=keycloak-bundle:0.0.1",
			"--set", "metadata.bundleFile=/tmp/keycloak-bundle-0.0.1.tar.gz",
		)
		if err != nil {
			t.Fatalf("resolve returned error: %v", err)
		}

		if service.resolveSelection.Name != "dev" {
			t.Fatalf("expected selection name dev, got %q", service.resolveSelection.Name)
		}
		if got := service.resolveSelection.Overrides["metadata.baseDir"]; got != "/tmp/meta" {
			t.Fatalf("expected metadata override to be forwarded, got %q", got)
		}
		if got := service.resolveSelection.Overrides["metadata.bundle"]; got != "keycloak-bundle:0.0.1" {
			t.Fatalf("expected metadata bundle override to be forwarded, got %q", got)
		}
		if got := service.resolveSelection.Overrides["metadata.bundleFile"]; got != "/tmp/keycloak-bundle-0.0.1.tar.gz" {
			t.Fatalf("expected metadata bundleFile override to be forwarded, got %q", got)
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
				Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
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
		{name: "list_json", format: cliutil.OutputJSON, commandArgs: []string{"list"}, expectedSnippet: "\"name\": \"dev\""},
		{name: "list_yaml", format: cliutil.OutputYAML, commandArgs: []string{"list"}, expectedSnippet: "- name: dev"},
		{name: "current_text", format: cliutil.OutputText, commandArgs: []string{"current"}, expectedSnippet: "dev\n"},
		{name: "current_json", format: cliutil.OutputJSON, commandArgs: []string{"current"}, expectedSnippet: "\"name\": \"dev\""},
		{name: "current_yaml", format: cliutil.OutputYAML, commandArgs: []string{"current"}, expectedSnippet: "name: dev"},
		{name: "resolve_text", format: cliutil.OutputText, commandArgs: []string{"resolve"}, expectedSnippet: "prod\n"},
		{name: "resolve_json", format: cliutil.OutputJSON, commandArgs: []string{"resolve"}, expectedSnippet: "\"name\": \"prod\""},
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
						Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
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
		Contexts: contextService,
		Services: &testConfigServiceAccessor{
			store:    &testRepositoryService{},
			sync:     &testRepositoryService{},
			metadata: &testMetadataService{},
		},
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
		"[SKIP] managedService",
		"[SKIP] secretStore",
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
		Contexts: contextService,
		Services: &testConfigServiceAccessor{
			store:    &testRepositoryService{},
			sync:     &testRepositoryService{},
			metadata: &testMetadataService{},
		},
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
		Contexts: contextService,
		Services: &testConfigServiceAccessor{
			store:    &testRepositoryService{},
			sync:     &testRepositoryService{},
			metadata: &testMetadataService{},
		},
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

func TestCheckWarnsForReachableManagedServiceProbeErrors(t *testing.T) {
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
			ManagedService: &configdomain.ManagedService{
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
		Contexts: contextService,
		Services: &testConfigServiceAccessor{
			store:    &testRepositoryService{},
			sync:     &testRepositoryService{},
			metadata: &testMetadataService{},
			server:   &testManagedServiceClientService{requestErr: faults.NewTypedError(faults.NotFoundError, "probe not found", nil)},
		},
	}
	globalFlags := &cliutil.GlobalFlags{Output: cliutil.OutputText}

	output, err := executeConfigCommandWithDeps(t, deps, globalFlags, "", "check")
	if err != nil {
		t.Fatalf("check returned error: %v", err)
	}
	if !strings.Contains(output, "[WARN] managedService") {
		t.Fatalf("expected warn status for managed service probe, got %q", output)
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
			ManagedService: &configdomain.ManagedService{
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
		Contexts:     contextService,
		Orchestrator: &testOrchestratorService{listRemoteErr: faults.NewTypedError(faults.AuthError, "managed service auth failed", nil)},
		Services: &testConfigServiceAccessor{
			store:    &testRepositoryService{},
			sync:     &testRepositoryService{},
			metadata: &testMetadataService{},
			secrets:  &testSecretProviderService{listErr: faults.NewTypedError(faults.TransportError, "secret store unavailable", nil)},
		},
	}
	globalFlags := &cliutil.GlobalFlags{Output: cliutil.OutputText}

	output, err := executeConfigCommandWithDeps(t, deps, globalFlags, "", "check")
	assertTypedCategory(t, err, faults.ValidationError)

	if !strings.Contains(output, "[FAIL] managedService") {
		t.Fatalf("expected managedService failure in output, got %q", output)
	}
	if !strings.Contains(output, "[FAIL] secretStore") {
		t.Fatalf("expected secretStore failure in output, got %q", output)
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
		Contexts: contextService,
		Services: &testConfigServiceAccessor{
			sync:     &testRepositoryService{},
			metadata: &testMetadataService{},
		},
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
		Contexts: contextService,
		Services: &testConfigServiceAccessor{
			sync:     repositoryService,
			metadata: metadataService,
		},
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
		Contexts: contextService,
		Services: &testConfigServiceAccessor{
			sync:     repositoryService,
			metadata: metadataService,
		},
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
		selects:     []string{"filesystem", "customHeaders"},
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
	if service.createdContext.Repository.Filesystem == nil || service.createdContext.Repository.Filesystem.BaseDir != "/tmp/repo" {
		t.Fatalf("unexpected repository config: %#v", service.createdContext.Repository)
	}
	if service.createdContext.Metadata.BaseDir != "/tmp/meta" {
		t.Fatalf("expected metadata baseDir /tmp/meta, got %q", service.createdContext.Metadata.BaseDir)
	}
	if service.createdContext.ManagedService == nil || service.createdContext.ManagedService.HTTP == nil {
		t.Fatal("expected managedService configuration")
	}
	if len(prompter.selectPrompts) == 0 || prompter.selectPrompts[0] != "Select repository type" {
		t.Fatalf("expected repository type prompt first, got %#v", prompter.selectPrompts)
	}
}

func TestCreateInteractivePromptFlowDefaultsMetadataBaseDirToRepoBaseDir(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"dev", "/tmp/repo", "", "https://api.example.com", "", "Authorization", "Bearer", "token-dev"},
		selects:     []string{"filesystem", "customHeaders"},
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
		t.Fatalf("expected metadata baseDir to default to repository baseDir /tmp/repo, got %q", service.createdContext.Metadata.BaseDir)
	}
	if len(prompter.inputPrompts) < 3 {
		t.Fatalf("expected at least 3 input prompts, got %d", len(prompter.inputPrompts))
	}
	if got := prompter.inputPrompts[2]; got != "Metadata baseDir (defaults to /tmp/repo): " {
		t.Fatalf("expected metadata prompt with repository baseDir value, got %q", got)
	}
}

func TestCreateInteractivePromptFlowSupportsManagedServiceProxy(t *testing.T) {
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
			"proxy-creds",
			"proxy-user",
			"proxy-pass",
			"Authorization",
			"Bearer",
			"token-dev",
		},
		selects: []string{
			"filesystem",
			"customHeaders",
		},
		confirms: []bool{
			false,
			true,
			true,
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

	if service.createdContext.ManagedService == nil || service.createdContext.ManagedService.HTTP == nil {
		t.Fatal("expected managedService configuration")
	}
	if service.createdContext.ManagedService.HTTP.Proxy == nil {
		t.Fatal("expected managedService proxy configuration")
	}

	proxy := service.createdContext.ManagedService.HTTP.Proxy
	if proxy.HTTPURL != "http://proxy.example.com:3128" {
		t.Fatalf("expected proxy http, got %q", proxy.HTTPURL)
	}
	if proxy.HTTPSURL != "" {
		t.Fatalf("expected empty proxy https, got %q", proxy.HTTPSURL)
	}
	if proxy.NoProxy != "localhost,127.0.0.1" {
		t.Fatalf("expected proxy noProxy, got %q", proxy.NoProxy)
	}
	if proxy.Auth == nil {
		t.Fatal("expected proxy auth configuration")
	}
	if proxy.Auth.Basic == nil ||
		proxy.Auth.Basic.CredentialName() != "proxy-creds" ||
		proxy.Auth.Basic.Username.Literal() != "proxy-user" ||
		proxy.Auth.Basic.Password.Literal() != "proxy-pass" {
		t.Fatalf("unexpected proxy auth values: %#v", proxy.Auth)
	}
	if service.createdContext.Credentials["proxy-creds"].Username.Literal() != "proxy-user" {
		t.Fatalf("expected stored proxy credential username, got %#v", service.createdContext.Credentials)
	}
}

func TestCreateInteractivePromptFlowUsesPositionalName(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"/tmp/repo", "/tmp/meta", "https://api.example.com", "", "Authorization", "Bearer", "token-dev"},
		selects:     []string{"filesystem", "customHeaders"},
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
	if got := prompter.inputPrompts[0]; got != "Repository baseDir: " {
		t.Fatalf("expected first prompt to skip context name and ask repository baseDir, got %q", got)
	}
}

func TestCreateInteractivePromptFlowUsesContextFlagName(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"/tmp/repo", "/tmp/meta", "https://api.example.com", "", "Authorization", "Bearer", "token-dev"},
		selects:     []string{"filesystem", "customHeaders"},
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
	if got := prompter.inputPrompts[0]; got != "Repository baseDir: " {
		t.Fatalf("expected first prompt to skip context name and ask repository baseDir, got %q", got)
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

func TestCreateInteractivePromptFlowOmitsRepositoryResourceFormat(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"dev", "/tmp/repo", "/tmp/meta", "https://api.example.com", "", "Authorization", "Bearer", "token-dev"},
		selects:     []string{"filesystem", "customHeaders"},
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
	if service.createdContext.Repository.Filesystem == nil {
		t.Fatalf("expected filesystem repository config, got %#v", service.createdContext.Repository)
	}
}

func TestCreateInteractivePromptFlowGitLocalAutoInitCanBeDisabled(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"dev", "/tmp/repo-git", "/tmp/meta", "https://api.example.com", "", "Authorization", "Bearer", "token-dev"},
		selects:     []string{"git", "customHeaders"},
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
		t.Fatal("expected git local autoInit to be disabled")
	}
	if service.createdContext.Repository.Git.Local.AutoInit == nil {
		t.Fatal("expected autoInit=false to be persisted explicitly")
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
			"clientID",
			"clientSecret",
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
			"filesystem",
			"oauth2",
			"file",
			"keyFile",
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
	if service.createdContext.ManagedService == nil || service.createdContext.ManagedService.HTTP == nil {
		t.Fatal("expected managedService http configuration")
	}
	if service.createdContext.ManagedService.HTTP.Auth == nil {
		t.Fatal("expected managedService auth configuration")
	}
	if service.createdContext.ManagedService.HTTP.Auth.OAuth2 == nil {
		t.Fatal("expected managedService oauth2 configuration")
	}
	if service.createdContext.ManagedService.HTTP.Auth.Basic != nil {
		t.Fatal("basic auth should not be configured when oauth2 is selected")
	}
	if len(service.createdContext.ManagedService.HTTP.Auth.CustomHeaders) != 0 {
		t.Fatal("customHeaders auth should not be configured when oauth2 is selected")
	}
	if service.createdContext.ManagedService.HTTP.Auth.OAuth2.GrantType != configdomain.OAuthClientCreds {
		t.Fatalf(
			"expected oauth2 grantType default %q, got %q",
			configdomain.OAuthClientCreds,
			service.createdContext.ManagedService.HTTP.Auth.OAuth2.GrantType,
		)
	}

	if service.createdContext.SecretStore == nil || service.createdContext.SecretStore.File == nil {
		t.Fatal("expected file secretStore configuration")
	}
	if service.createdContext.SecretStore.File.KeyFile != "/tmp/key.txt" {
		t.Fatalf("expected secretStore keyFile /tmp/key.txt, got %q", service.createdContext.SecretStore.File.KeyFile)
	}
	if service.createdContext.SecretStore.File.Key != "" {
		t.Fatal("secretStore key should not be set when keyFile source is selected")
	}
	if service.createdContext.SecretStore.File.Passphrase != "" || service.createdContext.SecretStore.File.PassphraseFile != "" {
		t.Fatal("secretStore passphrase fields should not be set when keyFile source is selected")
	}
	if service.createdContext.SecretStore.File.KDF == nil {
		t.Fatal("expected secretStore KDF configuration")
	}
	if service.createdContext.SecretStore.File.KDF.Time != 1 ||
		service.createdContext.SecretStore.File.KDF.Memory != 65536 ||
		service.createdContext.SecretStore.File.KDF.Threads != 4 {
		t.Fatalf("unexpected KDF values: %#v", service.createdContext.SecretStore.File.KDF)
	}

	if value := service.createdContext.Preferences["env"]; value != "dev" {
		t.Fatalf("expected preference env=dev, got %q", value)
	}
	if len(prompter.inputPrompts) == 0 || prompter.inputPrompts[0] != "Repository baseDir: " {
		t.Fatalf("expected first prompt to skip context name and ask repository baseDir, got %q", prompter.inputPrompts)
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
		catalogValue: configdomain.ContextCatalog{
			CurrentContext: "dev",
			Credentials: []configdomain.Credential{{
				Name:     "shared-proxy-auth",
				Username: configdomain.LiteralCredential("proxy-user"),
				Password: configdomain.LiteralCredential("proxy-pass"),
			}},
			Contexts: []configdomain.Context{{
				Name: "prod",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
				},
			}},
		},
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
	if !strings.Contains(output, "contexts:") || !strings.Contains(output, "name: prod") {
		t.Fatalf("expected one-context catalog output for prod, got %q", output)
	}
	if !strings.Contains(output, "currentContext: prod") {
		t.Fatalf("expected shown currentContext to be prod, got %q", output)
	}
	if !strings.Contains(output, "credentials:") {
		t.Fatalf("expected catalog-scoped credentials in show output, got %q", output)
	}
}

func TestShowUsesPositionalContextNameWhenProvided(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		catalogValue: configdomain.ContextCatalog{
			CurrentContext: "dev",
			Contexts: []configdomain.Context{{
				Name: "prod",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
				},
			}},
		},
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
	if !strings.Contains(output, "contexts:") || !strings.Contains(output, "name: prod") {
		t.Fatalf("expected one-context catalog output for prod, got %q", output)
	}
	if !strings.Contains(output, "currentContext: prod") {
		t.Fatalf("expected shown currentContext to be prod, got %q", output)
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
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/dev"},
				},
			},
			{
				Name: "prod",
				Repository: configdomain.Repository{
					Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
				},
			},
		},
		catalogValue: configdomain.ContextCatalog{
			CurrentContext: "prod",
			Contexts: []configdomain.Context{
				{
					Name: "dev",
					Repository: configdomain.Repository{
						Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/dev"},
					},
				},
				{
					Name: "prod",
					Repository: configdomain.Repository{
						Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
					},
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
	if !strings.Contains(output, "contexts:") || !strings.Contains(output, "name: dev") {
		t.Fatalf("expected one-context catalog output for dev, got %q", output)
	}
	if !strings.Contains(output, "currentContext: dev") {
		t.Fatalf("expected shown currentContext to be dev, got %q", output)
	}
}

func TestShowPreservesCatalogAttributesAndExplicitMetadataBaseDir(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		catalogValue: configdomain.ContextCatalog{
			CurrentContext: "prod",
			DefaultEditor:  "vim",
			Credentials: []configdomain.Credential{{
				Name: "shared-proxy-auth",
				Username: configdomain.CredentialValue{
					Prompt: &configdomain.CredentialPrompt{
						Prompt:           true,
						PersistInSession: true,
					},
				},
				Password: configdomain.CredentialValue{
					Prompt: &configdomain.CredentialPrompt{
						Prompt:           true,
						PersistInSession: true,
					},
				},
			}},
			Contexts: []configdomain.Context{
				{
					Name: "dev",
					Repository: configdomain.Repository{
						Filesystem: &configdomain.FilesystemRepository{BaseDir: "/tmp/repo"},
					},
					ManagedService: &configdomain.ManagedService{
						HTTP: &configdomain.HTTPServer{
							BaseURL: "https://example.com/api",
							Proxy: &configdomain.HTTPProxy{
								HTTPURL: "http://proxy.example.com:3128",
								Auth: &configdomain.ProxyAuth{
									Basic: &configdomain.BasicAuth{
										CredentialsRef: &configdomain.CredentialsRef{Name: "shared-proxy-auth"},
									},
								},
							},
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
	requiredSnippets := []string{
		"defaultEditor: vim",
		"credentials:",
		"prompt: true",
		"persistInSession: true",
		"contexts:",
		"name: dev",
		"currentContext: dev",
		"metadata:",
		"baseDir: /tmp/repo",
		"credentialsRef:",
		"name: shared-proxy-auth",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(output, snippet) {
			t.Fatalf("expected show output to contain %q, got %q", snippet, output)
		}
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
	catalogValue     configdomain.ContextCatalog

	createdContext  configdomain.Context
	createdContexts []configdomain.Context
	setCurrentName  string
	deletedName     string
	renameFrom      string
	renameTo        string
	replacedCatalog configdomain.ContextCatalog

	createCalled         bool
	updateCalled         bool
	validateCalled       bool
	resolveCalled        bool
	replaceCatalogCalled bool
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

func (s *testContextService) GetCatalog(context.Context) (configdomain.ContextCatalog, error) {
	return s.catalogValue, nil
}

func (s *testContextService) ReplaceCatalog(_ context.Context, catalog configdomain.ContextCatalog) error {
	s.replaceCatalogCalled = true
	s.replacedCatalog = catalog
	return nil
}

type testRepositoryService struct {
	initCalled    bool
	initErr       error
	checkErr      error
	syncStatusErr error
	syncStatus    repository.SyncReport
}

func (s *testRepositoryService) Save(context.Context, string, resource.Content) error { return nil }
func (s *testRepositoryService) Get(context.Context, string) (resource.Content, error) {
	return testConfigContent(map[string]any{}), nil
}
func (s *testRepositoryService) Delete(context.Context, string, repository.DeletePolicy) error {
	return nil
}
func (s *testRepositoryService) List(context.Context, string, repository.ListPolicy) ([]resource.Resource, error) {
	return nil, nil
}
func (s *testRepositoryService) Exists(context.Context, string) (bool, error) { return false, nil }
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

func (s *testOrchestratorService) GetLocal(context.Context, string) (resource.Content, error) {
	return resource.Content{}, nil
}
func (s *testOrchestratorService) GetRemote(context.Context, string) (resource.Content, error) {
	return resource.Content{}, nil
}
func (s *testOrchestratorService) Request(context.Context, managedservicedomain.RequestSpec) (resource.Content, error) {
	return resource.Content{}, nil
}
func (s *testOrchestratorService) GetOpenAPISpec(context.Context) (resource.Content, error) {
	return resource.Content{}, nil
}
func (s *testOrchestratorService) Save(context.Context, string, resource.Content) error {
	return nil
}
func (s *testOrchestratorService) Apply(context.Context, string, orchestratordomain.ApplyPolicy) (resource.Resource, error) {
	return resource.Resource{}, nil
}
func (s *testOrchestratorService) ApplyWithContent(context.Context, string, resource.Content, orchestratordomain.ApplyPolicy) (resource.Resource, error) {
	return resource.Resource{}, nil
}
func (s *testOrchestratorService) Create(context.Context, string, resource.Content) (resource.Resource, error) {
	return resource.Resource{}, nil
}
func (s *testOrchestratorService) Update(context.Context, string, resource.Content) (resource.Resource, error) {
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
func (s *testOrchestratorService) Diff(context.Context, string) ([]resource.DiffEntry, error) {
	return nil, nil
}
func (s *testOrchestratorService) Template(context.Context, string, resource.Content) (resource.Content, error) {
	return resource.Content{}, nil
}

type testManagedServiceClientService struct {
	requestErr error
}

func (s *testManagedServiceClientService) Get(context.Context, resource.Resource, metadatadomain.ResourceMetadata) (resource.Content, error) {
	return resource.Content{}, nil
}
func (s *testManagedServiceClientService) Create(context.Context, resource.Resource, metadatadomain.ResourceMetadata) (resource.Content, error) {
	return resource.Content{}, nil
}
func (s *testManagedServiceClientService) Update(context.Context, resource.Resource, metadatadomain.ResourceMetadata) (resource.Content, error) {
	return resource.Content{}, nil
}
func (s *testManagedServiceClientService) Delete(context.Context, resource.Resource, metadatadomain.ResourceMetadata) error {
	return nil
}
func (s *testManagedServiceClientService) List(context.Context, string, metadatadomain.ResourceMetadata) ([]resource.Resource, error) {
	return nil, nil
}
func (s *testManagedServiceClientService) Exists(context.Context, resource.Resource, metadatadomain.ResourceMetadata) (bool, error) {
	return false, nil
}
func (s *testManagedServiceClientService) Request(context.Context, managedservicedomain.RequestSpec) (resource.Content, error) {
	return resource.Content{}, s.requestErr
}
func (s *testManagedServiceClientService) GetOpenAPISpec(context.Context) (resource.Content, error) {
	return resource.Content{}, nil
}

type testSecretProviderService struct {
	listErr error
	keys    []string
}

func testConfigContent(value resource.Value) resource.Content {
	if value == nil {
		return resource.Content{}
	}
	return resource.Content{
		Value:      value,
		Descriptor: resource.NormalizePayloadDescriptor(resource.PayloadDescriptor{PayloadType: resource.PayloadTypeJSON}),
	}
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

type testConfigServiceAccessor struct {
	store    repository.ResourceStore
	sync     repository.RepositorySync
	metadata metadatadomain.MetadataService
	secrets  secretsdomain.SecretProvider
	server   managedservicedomain.ManagedServiceClient
}

func (a *testConfigServiceAccessor) RepositoryStore() repository.ResourceStore {
	return a.store
}
func (a *testConfigServiceAccessor) RepositorySync() repository.RepositorySync {
	return a.sync
}
func (a *testConfigServiceAccessor) MetadataService() metadatadomain.MetadataService {
	return a.metadata
}
func (a *testConfigServiceAccessor) SecretProvider() secretsdomain.SecretProvider {
	return a.secrets
}
func (a *testConfigServiceAccessor) ManagedServiceClient() managedservicedomain.ManagedServiceClient {
	return a.server
}

func promptAuthSessionFileName(sessionID string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(sessionID)))
	return "prompt-auth-" + hex.EncodeToString(digest[:8]) + ".json"
}

func writePromptAuthCacheFile(t *testing.T, path string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll(%q) returned error: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(`{"DECLAREST_CONTEXT_CREDENTIAL_SHARED_USERNAME":"user"}`), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", path, err)
	}
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
