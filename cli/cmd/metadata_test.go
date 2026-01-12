package cmd_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"declarest/internal/resource"
)

const cliInferSpecYAML = `
openapi: 3.0.0
paths:
  /fruits:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                id:
                  type: string
                name:
                  type: string
              required:
                - id
                - name
      responses:
        "201":
          description: created
          content:
            application/json: {}
  /fruits/{id}:
    get:
      responses:
        "200":
          description: ok
          content:
            application/json: {}
  /things:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                collectionId:
                  type: string
                displayName:
                  type: string
              required:
                - collectionId
      responses:
        "201":
          description: created
          content:
            application/json: {}
`

const cliInferWildcardSpecYAML = `
openapi: 3.0.0
paths:
  /admin/realms/{realm}/clients:
    post:
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                clientId:
                  type: string
                clientName:
                  type: string
              required:
                - clientId
      responses:
        "201":
          description: created
          content:
            application/json: {}
`

func TestMetadataGetPrintsSecretInAttributesWhenEmpty(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	metaPath := filepath.Join(repoDir, "items", "foo", "metadata.json")
	if err := os.MkdirAll(filepath.Dir(metaPath), 0o755); err != nil {
		t.Fatalf("mkdir metadata dir: %v", err)
	}
	if err := os.WriteFile(metaPath, []byte(`{"resourceInfo":{"secretInAttributes":[]}}`), 0o644); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "get")
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(io.Discard)

	if err := command.InheritedFlags().Set("for-resource-only", "true"); err != nil {
		t.Fatalf("set for-resource-only: %v", err)
	}

	if err := command.RunE(command, []string{"/items/foo"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !strings.Contains(out.String(), "\"secretInAttributes\": []") {
		t.Fatalf("expected secretInAttributes in output, got %q", out.String())
	}
}

func TestMetadataGetPrintsSecretInAttributesWhenUnset(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "get")
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(io.Discard)

	if err := command.InheritedFlags().Set("for-resource-only", "true"); err != nil {
		t.Fatalf("set for-resource-only: %v", err)
	}

	if err := command.RunE(command, []string{"/items/foo"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !strings.Contains(out.String(), "\"secretInAttributes\": []") {
		t.Fatalf("expected default secretInAttributes in output, got %q", out.String())
	}
}

func TestMetadataEditStripsDefaults(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "edit-test", contextPath)

	editorPath := filepath.Join(home, "edit.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail

if ! grep -q '"aliasFromAttribute": "id"' "$1"; then
  echo "missing default aliasFromAttribute" >&2
  exit 1
fi

sed -i 's/"aliasFromAttribute": "id"/"aliasFromAttribute": "name"/' "$1"
`
	if err := os.WriteFile(editorPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "edit")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("editor", editorPath); err != nil {
		t.Fatalf("set editor: %v", err)
	}

	if err := command.RunE(command, []string{"/items/"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, "items", "_", "metadata.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	var meta resource.ResourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.ResourceInfo == nil {
		t.Fatalf("expected resourceInfo metadata")
	}
	if meta.ResourceInfo.AliasFromAttribute != "name" {
		t.Fatalf("unexpected aliasFromAttribute: %q", meta.ResourceInfo.AliasFromAttribute)
	}
	if meta.ResourceInfo.IDFromAttribute != "" {
		t.Fatalf("expected idFromAttribute to be stripped, got %q", meta.ResourceInfo.IDFromAttribute)
	}
	if meta.ResourceInfo.CollectionPath != "" {
		t.Fatalf("expected collectionPath to be stripped, got %q", meta.ResourceInfo.CollectionPath)
	}
	if meta.OperationInfo != nil {
		t.Fatalf("expected default operationInfo to be stripped")
	}
}

func TestMetadataEditRejectsInvalidMetadata(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo-invalid-metadata")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "meta-invalid", contextPath)

	editorPath := filepath.Join(home, "invalid-metadata.sh")
	script := `#!/usr/bin/env bash
set -euo pipefail

cat > "$1" <<'EOF'
{
  "resourceInfo": {
    "secretInAttributes": "not-an-array"
  }
}
EOF
`
	if err := os.WriteFile(editorPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor script: %v", err)
	}

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "edit")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("editor", editorPath); err != nil {
		t.Fatalf("set editor: %v", err)
	}

	err := command.RunE(command, []string{"/items/"})
	if err == nil {
		t.Fatalf("expected metadata edit to reject invalid payload")
	}
	if !strings.Contains(err.Error(), "invalid metadata") {
		t.Fatalf("unexpected error: %v", err)
	}

	metaPath := filepath.Join(repoDir, "items", "_", "metadata.json")
	if _, statErr := os.Stat(metaPath); statErr == nil {
		t.Fatalf("metadata file should not be created after invalid edit")
	} else if !os.IsNotExist(statErr) {
		t.Fatalf("unexpected metadata stat error: %v", statErr)
	}
}

func TestMetadataSetSecretInAttributesCommaSeparated(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "set")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.InheritedFlags().Set("for-resource-only", "true"); err != nil {
		t.Fatalf("set for-resource-only: %v", err)
	}
	if err := command.Flags().Set("attribute", "resourceInfo.secretInAttributes"); err != nil {
		t.Fatalf("set attribute: %v", err)
	}
	if err := command.Flags().Set("value", "bla,ble,bli"); err != nil {
		t.Fatalf("set value: %v", err)
	}

	if err := command.RunE(command, []string{"/items/foo"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, "items", "foo", "metadata.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	var meta resource.ResourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.ResourceInfo == nil {
		t.Fatalf("expected resourceInfo metadata")
	}
	if !reflect.DeepEqual(meta.ResourceInfo.SecretInAttributes, []string{"bla", "ble", "bli"}) {
		t.Fatalf("unexpected secretInAttributes: %#v", meta.ResourceInfo.SecretInAttributes)
	}
}

func TestMetadataSetSecretInAttributesJSONArray(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "set")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.InheritedFlags().Set("for-resource-only", "true"); err != nil {
		t.Fatalf("set for-resource-only: %v", err)
	}
	if err := command.Flags().Set("attribute", "resourceInfo.secretInAttributes"); err != nil {
		t.Fatalf("set attribute: %v", err)
	}
	if err := command.Flags().Set("value", `["bla","ble"]`); err != nil {
		t.Fatalf("set value: %v", err)
	}

	if err := command.RunE(command, []string{"/items/foo"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, "items", "foo", "metadata.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	var meta resource.ResourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.ResourceInfo == nil {
		t.Fatalf("expected resourceInfo metadata")
	}
	if !reflect.DeepEqual(meta.ResourceInfo.SecretInAttributes, []string{"bla", "ble"}) {
		t.Fatalf("unexpected secretInAttributes: %#v", meta.ResourceInfo.SecretInAttributes)
	}
}

func TestMetadataSetDefaultsToCollectionPath(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "")
	addContext(t, "test", contextPath)

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "set")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("attribute", "resourceInfo.idFromAttribute"); err != nil {
		t.Fatalf("set attribute: %v", err)
	}
	if err := command.Flags().Set("value", "id"); err != nil {
		t.Fatalf("set value: %v", err)
	}

	if err := command.RunE(command, []string{"/items/foo"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoDir, "items", "foo", "_", "metadata.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	var meta resource.ResourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.ResourceInfo == nil {
		t.Fatalf("expected resourceInfo metadata")
	}
	if meta.ResourceInfo.IDFromAttribute != "id" {
		t.Fatalf("unexpected idFromAttribute: %q", meta.ResourceInfo.IDFromAttribute)
	}
}

func TestMetadataInferPrintsSuggestions(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "http://example.com")
	addContext(t, "infer-test", contextPath)

	specPath := filepath.Join(home, "openapi.yml")
	if err := os.WriteFile(specPath, []byte(cliInferSpecYAML), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "infer")
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("spec", specPath); err != nil {
		t.Fatalf("set spec: %v", err)
	}

	if err := command.RunE(command, []string{"/fruits/apple"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var payload struct {
		ResourceInfo struct {
			IDFromAttribute    string `json:"idFromAttribute"`
			AliasFromAttribute string `json:"aliasFromAttribute"`
		} `json:"resourceInfo"`
		Reasons []string `json:"reasons"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if payload.ResourceInfo.IDFromAttribute != "id" {
		t.Fatalf("unexpected idFromAttribute: %q", payload.ResourceInfo.IDFromAttribute)
	}
	if payload.ResourceInfo.AliasFromAttribute != "name" {
		t.Fatalf("unexpected aliasFromAttribute: %q", payload.ResourceInfo.AliasFromAttribute)
	}
	if len(payload.Reasons) == 0 {
		t.Fatalf("expected inference reasons, got none")
	}
}

func TestMetadataInferDefaultsToCollectionWithoutSlash(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "http://example.com")
	addContext(t, "infer-no-slash", contextPath)

	specPath := filepath.Join(home, "openapi.yml")
	if err := os.WriteFile(specPath, []byte(cliInferSpecYAML), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "infer")
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("spec", specPath); err != nil {
		t.Fatalf("set spec: %v", err)
	}

	if err := command.RunE(command, []string{"/things"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var payload struct {
		ResourceInfo struct {
			IDFromAttribute    string `json:"idFromAttribute"`
			AliasFromAttribute string `json:"aliasFromAttribute"`
		} `json:"resourceInfo"`
		Reasons []string `json:"reasons"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if payload.ResourceInfo.IDFromAttribute != "collectionId" {
		t.Fatalf("unexpected idFromAttribute: %q", payload.ResourceInfo.IDFromAttribute)
	}
	if payload.ResourceInfo.AliasFromAttribute != "displayName" {
		t.Fatalf("unexpected aliasFromAttribute: %q", payload.ResourceInfo.AliasFromAttribute)
	}
	if len(payload.Reasons) == 0 {
		t.Fatalf("expected inference reasons, got none")
	}
}

func TestMetadataInferApplyWritesMetadata(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	specPath := filepath.Join(home, "openapi.yml")
	if err := os.WriteFile(specPath, []byte(cliInferSpecYAML), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	content := fmt.Sprintf(`
repository:
  filesystem:
    base_dir: %s
managed_server:
  http:
    base_url: http://example.com
    openapi: %s
`, repoDir, specPath)
	if err := os.WriteFile(contextPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write context: %v", err)
	}
	addContext(t, "infer-apply", contextPath)

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "infer")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("apply", "true"); err != nil {
		t.Fatalf("set apply: %v", err)
	}

	if err := command.RunE(command, []string{"/things/"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	metaPath := filepath.Join(repoDir, "things", "_", "metadata.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	var meta resource.ResourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.ResourceInfo == nil {
		t.Fatalf("expected resourceInfo metadata")
	}
	if meta.ResourceInfo.IDFromAttribute != "collectionId" {
		t.Fatalf("unexpected idFromAttribute: %q", meta.ResourceInfo.IDFromAttribute)
	}
	if meta.ResourceInfo.AliasFromAttribute != "displayName" {
		t.Fatalf("unexpected aliasFromAttribute: %q", meta.ResourceInfo.AliasFromAttribute)
	}
}

func TestMetadataInferApplyUsesWildcardPath(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	specPath := filepath.Join(home, "openapi-wildcards.yml")
	if err := os.WriteFile(specPath, []byte(cliInferWildcardSpecYAML), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	content := fmt.Sprintf(`
repository:
  filesystem:
    base_dir: %s
managed_server:
  http:
    base_url: http://example.com
    openapi: %s
`, repoDir, specPath)
	if err := os.WriteFile(contextPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write context: %v", err)
	}
	addContext(t, "infer-wildcard", contextPath)

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "infer")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("apply", "true"); err != nil {
		t.Fatalf("set apply: %v", err)
	}
	if err := command.Flags().Set("spec", specPath); err != nil {
		t.Fatalf("set spec: %v", err)
	}

	if err := command.RunE(command, []string{"/admin/realms/publico/clients/"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	wildcardMetaPath := filepath.Join(repoDir, "admin", "realms", "_", "clients", "_", "metadata.json")
	data, err := os.ReadFile(wildcardMetaPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}

	var meta resource.ResourceMetadata
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal metadata: %v", err)
	}
	if meta.ResourceInfo == nil {
		t.Fatalf("expected resourceInfo metadata")
	}
	if meta.ResourceInfo.IDFromAttribute != "clientId" {
		t.Fatalf("unexpected idFromAttribute: %q", meta.ResourceInfo.IDFromAttribute)
	}

	literalMetaPath := filepath.Join(repoDir, "admin", "realms", "publico", "clients", "_", "metadata.json")
	if _, err := os.Stat(literalMetaPath); err == nil {
		t.Fatalf("unexpected metadata at literal realm path")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat literal metadata: %v", err)
	}
}

func TestMetadataInferRecursivelyPrintsSuggestions(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "http://example.com")
	addContext(t, "infer-recursive", contextPath)

	specPath := filepath.Join(home, "openapi.yml")
	if err := os.WriteFile(specPath, []byte(cliInferSpecYAML), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "infer")
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("spec", specPath); err != nil {
		t.Fatalf("set spec: %v", err)
	}
	if err := command.Flags().Set("recursively", "true"); err != nil {
		t.Fatalf("set recursively: %v", err)
	}

	if err := command.RunE(command, []string{"/"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var payload struct {
		Results []struct {
			Path         string `json:"path"`
			ResourceInfo struct {
				IDFromAttribute    string `json:"idFromAttribute"`
				AliasFromAttribute string `json:"aliasFromAttribute"`
			} `json:"resourceInfo"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(payload.Results) != 2 {
		t.Fatalf("expected 2 collection entries, got %d", len(payload.Results))
	}

	resultsByPath := make(map[string]struct {
		id    string
		alias string
	})
	for _, entry := range payload.Results {
		resultsByPath[entry.Path] = struct {
			id    string
			alias string
		}{
			id:    entry.ResourceInfo.IDFromAttribute,
			alias: entry.ResourceInfo.AliasFromAttribute,
		}
	}

	if got := resultsByPath["/fruits"].id; got != "id" {
		t.Fatalf("expected /fruits idFromAttribute id, got %q", got)
	}
	if got := resultsByPath["/fruits"].alias; got != "name" {
		t.Fatalf("expected /fruits aliasFromAttribute name, got %q", got)
	}
	if got := resultsByPath["/things"].id; got != "collectionId" {
		t.Fatalf("expected /things idFromAttribute collectionId, got %q", got)
	}
	if got := resultsByPath["/things"].alias; got != "displayName" {
		t.Fatalf("expected /things aliasFromAttribute displayName, got %q", got)
	}
	if len(resultsByPath) != 2 {
		t.Fatalf("expected only /fruits and /things entries, got %d", len(resultsByPath))
	}
}

func TestMetadataInferRecursivelyFiltersByPath(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	writeContextConfig(t, contextPath, repoDir, "http://example.com")
	addContext(t, "infer-recursive-filter", contextPath)

	specPath := filepath.Join(home, "openapi.yml")
	if err := os.WriteFile(specPath, []byte(cliInferSpecYAML), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "infer")
	var out bytes.Buffer
	command.SetOut(&out)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("spec", specPath); err != nil {
		t.Fatalf("set spec: %v", err)
	}
	if err := command.Flags().Set("recursively", "true"); err != nil {
		t.Fatalf("set recursively: %v", err)
	}

	if err := command.RunE(command, []string{"/things/"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	var payload struct {
		Results []struct {
			Path string `json:"path"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(payload.Results) != 1 {
		t.Fatalf("expected 1 entry for /things, got %d", len(payload.Results))
	}
	if payload.Results[0].Path != "/things" {
		t.Fatalf("expected /things entry, got %q", payload.Results[0].Path)
	}
}

func TestMetadataInferRecursivelyApplyWritesMetadata(t *testing.T) {
	home := setTempHome(t)
	repoDir := filepath.Join(home, "repo")
	contextPath := filepath.Join(home, "context.yaml")
	specPath := filepath.Join(home, "openapi.yml")
	if err := os.WriteFile(specPath, []byte(cliInferSpecYAML), 0o644); err != nil {
		t.Fatalf("write spec: %v", err)
	}

	content := fmt.Sprintf(`
repository:
  filesystem:
    base_dir: %s
managed_server:
  http:
    base_url: http://example.com
    openapi: %s
`, repoDir, specPath)
	if err := os.WriteFile(contextPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write context: %v", err)
	}
	addContext(t, "infer-recursive-apply", contextPath)

	root := newRootCommand()
	command := findCommand(t, root, "metadata", "infer")
	command.SetOut(io.Discard)
	command.SetErr(io.Discard)

	if err := command.Flags().Set("apply", "true"); err != nil {
		t.Fatalf("set apply: %v", err)
	}
	if err := command.Flags().Set("recursively", "true"); err != nil {
		t.Fatalf("set recursively: %v", err)
	}

	for _, relPath := range []string{
		filepath.Join("fruits", "_", "metadata.json"),
		filepath.Join("things", "_", "metadata.json"),
	} {
		_ = os.Remove(filepath.Join(repoDir, relPath))
	}

	if err := command.RunE(command, []string{"/"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}

	checkMetadata := func(rel string, expectedID, expectedAlias string) {
		metaPath := filepath.Join(repoDir, rel)
		data, err := os.ReadFile(metaPath)
		if err != nil {
			t.Fatalf("read metadata %s: %v", rel, err)
		}
		var meta resource.ResourceMetadata
		if err := json.Unmarshal(data, &meta); err != nil {
			t.Fatalf("unmarshal metadata %s: %v", rel, err)
		}
		if meta.ResourceInfo == nil {
			t.Fatalf("expected resourceInfo metadata in %s", rel)
		}
		if meta.ResourceInfo.IDFromAttribute != expectedID {
			t.Fatalf("unexpected idFromAttribute for %s: %q", rel, meta.ResourceInfo.IDFromAttribute)
		}
		if meta.ResourceInfo.AliasFromAttribute != expectedAlias {
			t.Fatalf("unexpected aliasFromAttribute for %s: %q", rel, meta.ResourceInfo.AliasFromAttribute)
		}
	}

	checkMetadata(filepath.Join("fruits", "_", "metadata.json"), "id", "name")
	checkMetadata(filepath.Join("things", "_", "metadata.json"), "collectionId", "displayName")
}
