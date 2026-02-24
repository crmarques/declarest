package debugctx

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type enabledKey struct{}
type writerKey struct{}

func WithEnabled(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, enabledKey{}, enabled)
}

func Enabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}

	enabled, _ := ctx.Value(enabledKey{}).(bool)
	return enabled
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

func Printf(ctx context.Context, format string, args ...any) {
	if !Enabled(ctx) {
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

	_, _ = fmt.Fprintf(writer, "debug: %s\n", message)
}
