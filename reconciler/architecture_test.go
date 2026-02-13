package reconciler_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

var anonymousAssertionPattern = regexp.MustCompile(`\.\(interface\s*\{`)

func TestReconcilerPackageAvoidsGoGitImports(t *testing.T) {
	root := repoRoot(t)
	files := goFiles(t, filepath.Join(root, "reconciler"))
	for _, file := range files {
		content := readFile(t, file)
		if strings.Contains(content, "github.com/go-git/go-git") {
			t.Fatalf("reconciler file imports go-git directly: %s", file)
		}
	}
}

func TestCorePackagesAvoidAnonymousTypeAssertions(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range []string{"context", "reconciler"} {
		files := goFiles(t, filepath.Join(root, dir))
		for _, file := range files {
			content := readFile(t, file)
			if anonymousAssertionPattern.MatchString(content) {
				t.Fatalf("anonymous interface assertion found in %s", file)
			}
		}
	}
}

func TestCLIConfigUsesContextManagerInterface(t *testing.T) {
	root := repoRoot(t)
	files := goFiles(t, filepath.Join(root, "cli", "cmd"))
	for _, file := range files {
		content := readFile(t, file)
		if strings.Contains(content, "*ctx.DefaultContextManager") {
			t.Fatalf("cli/cmd file references concrete context manager type: %s", file)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve current file path")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), ".."))
	return root
}

func goFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	var files []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, filepath.Join(dir, name))
	}
	return files
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %s: %v", path, err)
	}
	return string(data)
}
