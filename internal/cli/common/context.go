package common

import "context"

type contextNameKey struct{}

func WithContextName(ctx context.Context, name string) context.Context {
	return context.WithValue(ctx, contextNameKey{}, name)
}

func ContextName(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	name, _ := ctx.Value(contextNameKey{}).(string)
	return name
}
