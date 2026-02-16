package config

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

func TestCreateUpdateValidateRejectUnknownFields(t *testing.T) {
	t.Parallel()

	t.Run("create_rejects_unknown_json_field", func(t *testing.T) {
		t.Parallel()

		service := &testContextService{}
		_, err := executeConfigCommand(t, service, &common.GlobalFlags{}, `{
  "name": "dev",
  "repository": {"filesystem": {"base-dir": "/tmp/repo"}},
  "unknown": true
}`, "create")
		assertTypedCategory(t, err, faults.ValidationError)
		if service.createCalled {
			t.Fatal("expected create service call to be skipped on decode failure")
		}
	})

	t.Run("update_rejects_unknown_yaml_field", func(t *testing.T) {
		t.Parallel()

		service := &testContextService{}
		_, err := executeConfigCommand(t, service, &common.GlobalFlags{}, `
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
		_, err := executeConfigCommand(t, service, &common.GlobalFlags{}, `{
  "name": "dev",
  "repository": {"filesystem": {"base-dir": "/tmp/repo", "extra": true}}
}`, "validate")
		assertTypedCategory(t, err, faults.ValidationError)
		if service.validateCalled {
			t.Fatal("expected validate service call to be skipped on decode failure")
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
		globalFlags := &common.GlobalFlags{
			Context: "dev",
			Output:  common.OutputText,
		}

		_, err := executeConfigCommand(
			t,
			service,
			globalFlags,
			"",
			"resolve",
			"--set", "metadata.base-dir=/tmp/meta",
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
		if got := service.resolveSelection.Overrides["repository.resource-format"]; got != "yaml" {
			t.Fatalf("expected resource format override to be forwarded, got %q", got)
		}
	})

	t.Run("invalid_override_token_fails_validation", func(t *testing.T) {
		t.Parallel()

		service := &testContextService{}
		_, err := executeConfigCommand(t, service, &common.GlobalFlags{}, "", "resolve", "--set", "missing-equals")
		assertTypedCategory(t, err, faults.ValidationError)
		if service.resolveCalled {
			t.Fatal("expected resolve service call to be skipped on override parse failure")
		}
	})
}

func TestConfigOutputAcrossFormats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		format          string
		commandArgs     []string
		expectedSnippet string
	}{
		{name: "list_text", format: common.OutputText, commandArgs: []string{"list"}, expectedSnippet: "dev\nprod\n"},
		{name: "list_json", format: common.OutputJSON, commandArgs: []string{"list"}, expectedSnippet: "\"Name\": \"dev\""},
		{name: "list_yaml", format: common.OutputYAML, commandArgs: []string{"list"}, expectedSnippet: "- name: dev"},
		{name: "current_text", format: common.OutputText, commandArgs: []string{"current"}, expectedSnippet: "dev\n"},
		{name: "current_json", format: common.OutputJSON, commandArgs: []string{"current"}, expectedSnippet: "\"Name\": \"dev\""},
		{name: "current_yaml", format: common.OutputYAML, commandArgs: []string{"current"}, expectedSnippet: "name: dev"},
		{name: "resolve_text", format: common.OutputText, commandArgs: []string{"resolve"}, expectedSnippet: "prod\n"},
		{name: "resolve_json", format: common.OutputJSON, commandArgs: []string{"resolve"}, expectedSnippet: "\"Name\": \"prod\""},
		{name: "resolve_yaml", format: common.OutputYAML, commandArgs: []string{"resolve"}, expectedSnippet: "name: prod"},
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

			globalFlags := &common.GlobalFlags{
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

func TestCreateInteractivePromptFlow(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"dev", "/tmp/repo", "/tmp/meta"},
		selects:     []string{"filesystem"},
	}

	_, err := executeConfigCommandWithPrompter(
		t,
		service,
		&common.GlobalFlags{},
		prompter,
		"",
		"create",
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
}

func TestCreateInteractivePromptFlowDefaultsMetadataBaseDirToRepoBaseDir(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{
		interactive: true,
		inputs:      []string{"dev", "/tmp/repo", ""},
		selects:     []string{"filesystem"},
	}

	_, err := executeConfigCommandWithPrompter(
		t,
		service,
		&common.GlobalFlags{},
		prompter,
		"",
		"create",
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

func TestUseInteractiveSelection(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{{Name: "dev"}, {Name: "prod"}},
	}
	prompter := &mockPrompter{
		interactive: true,
		selects:     []string{"prod"},
	}

	_, err := executeConfigCommandWithPrompter(t, service, &common.GlobalFlags{}, prompter, "", "use")
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
		resolveValue: configdomain.Context{
			Name: "prod",
			Repository: configdomain.Repository{
				ResourceFormat: configdomain.ResourceFormatYAML,
				Filesystem:     &configdomain.FilesystemRepository{BaseDir: "/tmp/prod"},
			},
		},
	}
	prompter := &mockPrompter{interactive: true}
	globalFlags := &common.GlobalFlags{
		Context: "prod",
		Output:  common.OutputText,
	}

	output, err := executeConfigCommandWithPrompter(t, service, globalFlags, prompter, "", "show")
	if err != nil {
		t.Fatalf("show returned error: %v", err)
	}
	if service.resolveSelection.Name != "prod" {
		t.Fatalf("expected show to resolve context prod, got %q", service.resolveSelection.Name)
	}
	if !strings.Contains(output, "name: prod") {
		t.Fatalf("expected YAML output with context name prod, got %q", output)
	}
	if !strings.Contains(output, "resource-format: yaml") {
		t.Fatalf("expected YAML output for full context config, got %q", output)
	}
}

func TestShowInteractiveSelectionWhenContextFlagMissing(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{{Name: "dev"}, {Name: "prod"}},
		resolveValue: configdomain.Context{
			Name: "dev",
			Repository: configdomain.Repository{
				ResourceFormat: configdomain.ResourceFormatJSON,
				Filesystem:     &configdomain.FilesystemRepository{BaseDir: "/tmp/dev"},
			},
		},
	}
	prompter := &mockPrompter{
		interactive: true,
		selects:     []string{"dev"},
	}
	globalFlags := &common.GlobalFlags{Output: common.OutputText}

	output, err := executeConfigCommandWithPrompter(t, service, globalFlags, prompter, "", "show")
	if err != nil {
		t.Fatalf("show returned error: %v", err)
	}
	if service.resolveSelection.Name != "dev" {
		t.Fatalf("expected interactive show to resolve context dev, got %q", service.resolveSelection.Name)
	}
	if !strings.Contains(output, "name: dev") {
		t.Fatalf("expected YAML output with context name dev, got %q", output)
	}
	if !strings.Contains(output, "resource-format: json") {
		t.Fatalf("expected YAML output for full context config, got %q", output)
	}
}

func TestShowRequiresContextInNonInteractiveModeWhenFlagMissing(t *testing.T) {
	t.Parallel()

	service := &testContextService{
		listValue: []configdomain.Context{{Name: "dev"}},
	}
	prompter := &mockPrompter{interactive: false}
	globalFlags := &common.GlobalFlags{Output: common.OutputText}

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

	_, err := executeConfigCommandWithPrompter(t, service, &common.GlobalFlags{}, prompter, "", "rename")
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

	_, err := executeConfigCommandWithPrompter(t, service, &common.GlobalFlags{}, prompter, "", "delete")
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

	output, err := executeConfigCommandWithPrompter(t, service, &common.GlobalFlags{}, prompter, "", "delete")
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

			_, err := executeConfigCommandWithPrompter(t, service, &common.GlobalFlags{}, prompter, "", tt.args...)
			assertTypedCategory(t, err, faults.ValidationError)
		})
	}
}

func TestUseRenameDeleteWithArgsBypassInteractive(t *testing.T) {
	t.Parallel()

	service := &testContextService{}
	prompter := &mockPrompter{interactive: false}

	if _, err := executeConfigCommandWithPrompter(t, service, &common.GlobalFlags{}, prompter, "", "use", "prod"); err != nil {
		t.Fatalf("use returned error: %v", err)
	}
	if service.setCurrentName != "prod" {
		t.Fatalf("expected set current prod, got %q", service.setCurrentName)
	}

	if _, err := executeConfigCommandWithPrompter(t, service, &common.GlobalFlags{}, prompter, "", "rename", "dev", "development"); err != nil {
		t.Fatalf("rename returned error: %v", err)
	}
	if service.renameFrom != "dev" || service.renameTo != "development" {
		t.Fatalf("unexpected rename call: %q -> %q", service.renameFrom, service.renameTo)
	}

	if _, err := executeConfigCommandWithPrompter(t, service, &common.GlobalFlags{}, prompter, "", "delete", "legacy"); err != nil {
		t.Fatalf("delete returned error: %v", err)
	}
	if service.deletedName != "legacy" {
		t.Fatalf("expected delete legacy, got %q", service.deletedName)
	}
}

func executeConfigCommand(
	t *testing.T,
	contexts configdomain.ContextService,
	globalFlags *common.GlobalFlags,
	stdin string,
	args ...string,
) (string, error) {
	t.Helper()

	command := NewCommand(common.CommandDependencies{Contexts: contexts}, globalFlags)
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
	globalFlags *common.GlobalFlags,
	prompter configPrompter,
	stdin string,
	args ...string,
) (string, error) {
	t.Helper()

	command := newCommandWithPrompter(common.CommandDependencies{Contexts: contexts}, globalFlags, prompter)
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

	createdContext configdomain.Context
	setCurrentName string
	deletedName    string
	renameFrom     string
	renameTo       string

	createCalled   bool
	updateCalled   bool
	validateCalled bool
	resolveCalled  bool
}

func (s *testContextService) Create(_ context.Context, cfg configdomain.Context) error {
	s.createCalled = true
	s.createdContext = cfg
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
