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
