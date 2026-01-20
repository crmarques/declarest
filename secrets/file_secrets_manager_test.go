package secrets

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestFileSecretsManagerRoundTripWithRawKey(t *testing.T) {
	dir := t.TempDir()
	cfg := &FileSecretsManagerConfig{
		Path: filepath.Join(dir, "secrets.json"),
		Key:  testRawKey(t),
	}

	manager := NewFileSecretsManager(cfg)
	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if err := manager.CreateSecret("/items/foo", "password", "p"); err != nil {
		t.Fatalf("CreateSecret: %v", err)
	}

	if val, err := manager.GetSecret("/items/foo", "password"); err != nil || val != "p" {
		t.Fatalf("GetSecret: got %q err=%v", val, err)
	}

	if keys := manager.ListKeys("/items/foo"); len(keys) != 1 || keys[0] != "password" {
		t.Fatalf("ListKeys: unexpected keys %#v", keys)
	}

	resources, err := manager.ListResources()
	if err != nil {
		t.Fatalf("ListResources: %v", err)
	}
	if len(resources) != 1 || resources[0] != "/items/foo" {
		t.Fatalf("ListResources: unexpected resources %#v", resources)
	}

	info, err := os.Stat(cfg.Path)
	if err != nil {
		t.Fatalf("Stat secrets file: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("expected secrets file permissions to be 0600, got %v", info.Mode().Perm())
	}

	manager.Close()

	clone := NewFileSecretsManager(cfg)
	if err := clone.Init(); err != nil {
		t.Fatalf("Init (reopen): %v", err)
	}
	if val, err := clone.GetSecret("/items/foo", "password"); err != nil || val != "p" {
		t.Fatalf("GetSecret (reopen): got %q err=%v", val, err)
	}

	if err := clone.DeleteSecret("/items/foo", "password", ""); err != nil {
		t.Fatalf("DeleteSecret: %v", err)
	}
	if _, err := clone.GetSecret("/items/foo", "password"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected deleted secret to be missing, got err=%v", err)
	}
}

func TestFileSecretsManagerRoundTripWithPassphrase(t *testing.T) {
	dir := t.TempDir()
	cfg := &FileSecretsManagerConfig{
		Path:       filepath.Join(dir, "secrets.json"),
		Passphrase: "test-passphrase",
		KDF: &FileSecretsManagerKDFConfig{
			Time:    1,
			Memory:  8 * 1024,
			Threads: 1,
		},
	}

	manager := NewFileSecretsManager(cfg)
	if err := manager.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := manager.UpdateSecret("/items/bar", "token", "t"); err != nil {
		t.Fatalf("UpdateSecret: %v", err)
	}
	manager.Close()

	reopen := NewFileSecretsManager(cfg)
	if err := reopen.Init(); err != nil {
		t.Fatalf("Init (reopen): %v", err)
	}
	if val, err := reopen.GetSecret("/items/bar", "token"); err != nil || val != "t" {
		t.Fatalf("GetSecret (reopen): got %q err=%v", val, err)
	}
}

func TestFileSecretsManagerKeySourceValidation(t *testing.T) {
	dir := t.TempDir()
	cfg := &FileSecretsManagerConfig{
		Path:       filepath.Join(dir, "secrets.json"),
		Key:        testRawKey(t),
		Passphrase: "ignored",
	}

	manager := NewFileSecretsManager(cfg)
	if err := manager.Init(); err == nil {
		t.Fatalf("expected Init to fail when key and passphrase are both set")
	}
}

func testRawKey(t *testing.T) string {
	t.Helper()
	raw := bytes.Repeat([]byte{0x11}, fileSecretsKeyLen)
	return base64.StdEncoding.EncodeToString(raw)
}
