package debugctx_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/crmarques/declarest/debugctx"
)

func TestLevelDefaults(t *testing.T) {
	t.Parallel()

	if debugctx.Level(context.Background()) != 0 {
		t.Fatal("expected default level 0")
	}
	if debugctx.Level(context.TODO()) != 0 {
		t.Fatal("expected nil context level 0")
	}
}

func TestWithLevelClamps(t *testing.T) {
	t.Parallel()

	ctx := debugctx.WithLevel(context.Background(), -5)
	if debugctx.Level(ctx) != 0 {
		t.Fatalf("expected clamped to 0, got %d", debugctx.Level(ctx))
	}

	ctx = debugctx.WithLevel(context.Background(), 99)
	if debugctx.Level(ctx) != 3 {
		t.Fatalf("expected clamped to 3, got %d", debugctx.Level(ctx))
	}
}

func TestPrintfOnlyAtLevel3(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := debugctx.WithWriter(context.Background(), &buf)

	ctx2 := debugctx.WithLevel(ctx, 2)
	debugctx.Printf(ctx2, "should not appear")
	if buf.Len() != 0 {
		t.Fatalf("Printf should not output at level 2, got %q", buf.String())
	}

	ctx3 := debugctx.WithLevel(ctx, 3)
	debugctx.Printf(ctx3, "trace message")
	if !strings.Contains(buf.String(), "debug: trace message") {
		t.Fatalf("expected 'debug: trace message', got %q", buf.String())
	}
}

func TestInfofAtLevel1(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := debugctx.WithWriter(context.Background(), &buf)

	ctx0 := debugctx.WithLevel(ctx, 0)
	debugctx.Infof(ctx0, "should not appear")
	if buf.Len() != 0 {
		t.Fatalf("Infof should not output at level 0, got %q", buf.String())
	}

	ctx1 := debugctx.WithLevel(ctx, 1)
	debugctx.Infof(ctx1, "info message")
	if !strings.Contains(buf.String(), "verbose: info message") {
		t.Fatalf("expected 'verbose: info message', got %q", buf.String())
	}
}

func TestDetailfAtLevel2(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := debugctx.WithWriter(context.Background(), &buf)

	ctx1 := debugctx.WithLevel(ctx, 1)
	debugctx.Detailf(ctx1, "should not appear")
	if buf.Len() != 0 {
		t.Fatalf("Detailf should not output at level 1, got %q", buf.String())
	}

	ctx2 := debugctx.WithLevel(ctx, 2)
	debugctx.Detailf(ctx2, "detail message")
	if !strings.Contains(buf.String(), "verbose: detail message") {
		t.Fatalf("expected 'verbose: detail message', got %q", buf.String())
	}
}

func TestInsecureDefault(t *testing.T) {
	t.Parallel()

	if debugctx.Insecure(context.Background()) {
		t.Fatal("expected insecure to be false by default")
	}
	if debugctx.Insecure(context.TODO()) {
		t.Fatal("expected insecure to be false for nil context")
	}
}

func TestWithInsecure(t *testing.T) {
	t.Parallel()

	ctx := debugctx.WithInsecure(context.Background(), true)
	if !debugctx.Insecure(ctx) {
		t.Fatal("expected insecure to be true")
	}

	ctx = debugctx.WithInsecure(ctx, false)
	if debugctx.Insecure(ctx) {
		t.Fatal("expected insecure to be false after reset")
	}
}

func TestHigherLevelIncludesLower(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := debugctx.WithWriter(context.Background(), &buf)
	ctx = debugctx.WithLevel(ctx, 3)

	debugctx.Infof(ctx, "info")
	debugctx.Detailf(ctx, "detail")
	debugctx.Printf(ctx, "trace")

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines at level 3, got %d: %q", len(lines), output)
	}
	if !strings.Contains(lines[0], "verbose: info") {
		t.Fatalf("expected info line, got %q", lines[0])
	}
	if !strings.Contains(lines[1], "verbose: detail") {
		t.Fatalf("expected detail line, got %q", lines[1])
	}
	if !strings.Contains(lines[2], "debug: trace") {
		t.Fatalf("expected trace line, got %q", lines[2])
	}
}

func TestEmptyMessageSuppressed(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	ctx := debugctx.WithWriter(context.Background(), &buf)
	ctx = debugctx.WithLevel(ctx, 3)

	debugctx.Printf(ctx, "  ")
	debugctx.Infof(ctx, "")
	debugctx.Detailf(ctx, "   ")

	if buf.Len() != 0 {
		t.Fatalf("expected no output for empty messages, got %q", buf.String())
	}
}
