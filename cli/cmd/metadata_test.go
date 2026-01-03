package cmd_test

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"declarest/internal/resource"
)

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
