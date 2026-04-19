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

package debugctx

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type levelKey struct{}
type writerKey struct{}
type insecureKey struct{}

// WithLevel sets the verbosity level in the context.
//
//	0 = silent (default)
//	1 = info: enriched errors, mutation responses
//	2 = detail: HTTP request/response summaries, timing, auth events
//	3 = trace: full debug output, headers, bodies, metadata resolution
func WithLevel(ctx context.Context, level int) context.Context {
	if level < 0 {
		level = 0
	}
	if level > 3 {
		level = 3
	}
	return context.WithValue(ctx, levelKey{}, level)
}

// Level returns the verbosity level from the context.
func Level(ctx context.Context) int {
	if ctx == nil {
		return 0
	}
	level, _ := ctx.Value(levelKey{}).(int)
	return level
}

func WithWriter(ctx context.Context, writer io.Writer) context.Context {
	if writer == nil {
		return ctx
	}

	return context.WithValue(ctx, writerKey{}, writer)
}

func Writer(ctx context.Context) io.Writer {
	if ctx == nil {
		return nil
	}

	writer, _ := ctx.Value(writerKey{}).(io.Writer)
	return writer
}

// WithInsecure marks the context for insecure verbose output.
// When enabled, secrets, tokens, and credentials are printed without redaction.
func WithInsecure(ctx context.Context, insecure bool) context.Context {
	return context.WithValue(ctx, insecureKey{}, insecure)
}

// Insecure returns true if insecure verbose output is enabled.
func Insecure(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	insecure, _ := ctx.Value(insecureKey{}).(bool)
	return insecure
}

func Printf(ctx context.Context, format string, args ...any) {
	printAt(ctx, 3, "debug", format, args...)
}

func Infof(ctx context.Context, format string, args ...any) {
	printAt(ctx, 1, "verbose", format, args...)
}

func Detailf(ctx context.Context, format string, args ...any) {
	printAt(ctx, 2, "verbose", format, args...)
}

func printAt(ctx context.Context, minLevel int, prefix string, format string, args ...any) {
	if Level(ctx) < minLevel {
		return
	}

	writer := Writer(ctx)
	if writer == nil {
		return
	}

	message := strings.TrimSpace(fmt.Sprintf(format, args...))
	if message == "" {
		return
	}

	_, _ = fmt.Fprintf(writer, "%s: %s\n", prefix, message)
}
