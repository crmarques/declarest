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

package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/creack/pty"
)

func TestPromptAuthSessionHookIntegration(t *testing.T) {
	testCases := []struct {
		name          string
		shell         string
		args          []string
		skipIfMissing bool
	}{
		{
			name:  "bash",
			shell: "bash",
			args:  []string{"--noprofile", "--norc", "-i"},
		},
		{
			name:          "zsh",
			shell:         "zsh",
			args:          []string{"-f", "-i"},
			skipIfMissing: true,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			shellPath, err := exec.LookPath(testCase.shell)
			if err != nil {
				if testCase.skipIfMissing {
					t.Skipf("%s is not available: %v", testCase.shell, err)
				}
				t.Fatalf("failed to resolve %s: %v", testCase.shell, err)
			}

			binPath := buildDeclarestBinary(t)
			binDir := filepath.Dir(binPath)
			runtimeDir := t.TempDir()
			homeDir := t.TempDir()
			contextsPath := filepath.Join(t.TempDir(), "contexts.yaml")

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				user, pass, ok := r.BasicAuth()
				if !ok || user != "demo-user" || pass != "demo-pass" {
					w.Header().Set("WWW-Authenticate", `Basic realm="declarest"`)
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"ok":true}`))
			}))
			defer server.Close()

			writePromptAuthContextFile(t, contextsPath, server.URL)

			firstScript := strings.Join([]string{
				fmt.Sprintf(`eval "$(declarest context session-hook %s)"`, testCase.shell),
				"declarest server check",
				`printf "__COMMAND1_DONE__\n"`,
				"read -r __declarest_continue",
				"declarest server check",
				`printf "__COMMAND2_DONE__\n"`,
			}, "\n")

			firstSession := startInteractiveShell(
				t,
				shellPath,
				append(append([]string{}, testCase.args...), "-c", firstScript),
				binDir,
				homeDir,
				runtimeDir,
				contextsPath,
			)
			defer firstSession.Close()

			firstSession.waitFor(`credential "shared" username`, 20*time.Second)
			firstSession.writeLine("demo-user")
			time.Sleep(150 * time.Millisecond)
			firstSession.writeLine("demo-pass")
			firstSession.waitFor("__COMMAND1_DONE__", 20*time.Second)

			runtimeFiles := listPromptAuthRuntimeFiles(t, runtimeDir)
			if len(runtimeFiles) != 1 {
				t.Fatalf("expected one runtime cache file during shell session, got %d (%v)", len(runtimeFiles), runtimeFiles)
			}

			firstOutputAtBoundary := firstSession.Output()
			firstSession.writeLine("")
			firstSession.waitFor("__COMMAND2_DONE__", 20*time.Second)
			firstExitOutput, err := firstSession.Wait(20 * time.Second)
			if err != nil {
				t.Fatalf("first shell returned error: %v\noutput:\n%s", err, firstExitOutput)
			}

			laterOutput := strings.TrimPrefix(firstExitOutput, firstOutputAtBoundary)
			if strings.Contains(laterOutput, `credential "shared" username`) || strings.Contains(laterOutput, `credential "shared" password`) {
				t.Fatalf("expected second command in same shell to reuse credentials, got output:\n%s", laterOutput)
			}

			if runtimeFiles := listPromptAuthRuntimeFiles(t, runtimeDir); len(runtimeFiles) != 0 {
				t.Fatalf("expected runtime cache files to be removed on shell exit, got %v", runtimeFiles)
			}

			secondScript := strings.Join([]string{
				fmt.Sprintf(`eval "$(declarest context session-hook %s)"`, testCase.shell),
				"declarest server check",
				`printf "__NEW_SESSION_DONE__\n"`,
			}, "\n")

			secondSession := startInteractiveShell(
				t,
				shellPath,
				append(append([]string{}, testCase.args...), "-c", secondScript),
				binDir,
				homeDir,
				runtimeDir,
				contextsPath,
			)
			defer secondSession.Close()

			secondSession.waitFor(`credential "shared" username`, 20*time.Second)
			secondSession.writeLine("demo-user")
			time.Sleep(150 * time.Millisecond)
			secondSession.writeLine("demo-pass")
			secondSession.waitFor("__NEW_SESSION_DONE__", 20*time.Second)
			secondExitOutput, err := secondSession.Wait(20 * time.Second)
			if err != nil {
				t.Fatalf("second shell returned error: %v\noutput:\n%s", err, secondExitOutput)
			}
			if !strings.Contains(secondExitOutput, `credential "shared" username`) {
				t.Fatalf("expected a new shell session to prompt again, got output:\n%s", secondExitOutput)
			}
		})
	}
}

type interactiveShellSession struct {
	tb   testing.TB
	file *os.File
	cmd  *exec.Cmd

	done chan error

	mu  sync.Mutex
	buf bytes.Buffer
}

func startInteractiveShell(
	t *testing.T,
	shellPath string,
	args []string,
	binDir string,
	homeDir string,
	runtimeDir string,
	contextsPath string,
) *interactiveShellSession {
	t.Helper()

	cmd := exec.Command(shellPath, args...)
	cmd.Env = append(
		os.Environ(),
		"TERM=xterm-256color",
		"HOME="+homeDir,
		"XDG_RUNTIME_DIR="+runtimeDir,
		"DECLAREST_CONTEXTS_FILE="+contextsPath,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
	)

	file, err := pty.Start(cmd)
	if err != nil {
		t.Fatalf("failed to start PTY shell: %v", err)
	}

	session := &interactiveShellSession{
		tb:   t,
		file: file,
		cmd:  cmd,
		done: make(chan error, 1),
	}
	go func() {
		_, copyErr := io.Copy(session, file)
		session.done <- copyErr
	}()
	return session
}

func (s *interactiveShellSession) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *interactiveShellSession) Close() {
	if s == nil || s.file == nil {
		return
	}
	_ = s.file.Close()
	s.file = nil
}

func (s *interactiveShellSession) Output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return normalizeTerminalOutput(s.buf.String())
}

func (s *interactiveShellSession) waitFor(needle string, timeout time.Duration) {
	s.tb.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(s.Output(), needle) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	s.tb.Fatalf("timed out waiting for %q\noutput:\n%s", needle, s.Output())
}

func (s *interactiveShellSession) writeLine(line string) {
	if s == nil || s.file == nil {
		return
	}
	_, _ = io.WriteString(s.file, line+"\r")
}

func (s *interactiveShellSession) Wait(timeout time.Duration) (string, error) {
	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()

	select {
	case err := <-done:
		select {
		case <-s.done:
		default:
		}
		return s.Output(), err
	case <-time.After(timeout):
		_ = s.cmd.Process.Kill()
		_, _ = s.cmd.Process.Wait()
		return s.Output(), fmt.Errorf("timed out waiting for shell exit")
	}
}

func buildDeclarestBinary(t *testing.T) string {
	t.Helper()

	packageDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() returned error: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(packageDir, "..", ".."))
	binPath := filepath.Join(t.TempDir(), "declarest")

	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/declarest")
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build returned error: %v\noutput:\n%s", err, string(output))
	}
	return binPath
}

func writePromptAuthContextFile(t *testing.T, path string, serverURL string) {
	t.Helper()

	content := strings.Join([]string{
		"currentContext: shell",
		"credentials:",
		"  - name: shared",
		"    username:",
		"      prompt: true",
		"      persistInSession: true",
		"    password:",
		"      prompt: true",
		"      persistInSession: true",
		"contexts:",
		"  - name: shell",
		"    managedService:",
		"      http:",
		"        url: " + serverURL,
		"        auth:",
		"          basic:",
		"            credentialsRef:",
		"              name: shared",
		"",
	}, "\n")

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) returned error: %v", path, err)
	}
}

func listPromptAuthRuntimeFiles(t *testing.T, runtimeDir string) []string {
	t.Helper()

	promptAuthDir := filepath.Join(runtimeDir, "declarest", "prompt-auth")
	entries, err := os.ReadDir(promptAuthDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatalf("ReadDir(%q) returned error: %v", promptAuthDir, err)
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		files = append(files, entry.Name())
	}
	return files
}

var ansiEscapePattern = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func normalizeTerminalOutput(value string) string {
	value = ansiEscapePattern.ReplaceAllString(value, "")
	value = strings.ReplaceAll(value, "\r", "\n")
	return value
}
