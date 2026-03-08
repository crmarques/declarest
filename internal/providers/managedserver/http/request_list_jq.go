package http

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/itchyny/gojq"

	"github.com/crmarques/declarest/faults"
	managedserverdomain "github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/resource"
)

const maxJQCacheEntries = 128

var (
	jqCacheMu    sync.Mutex
	jqCacheMap   = make(map[string]*gojq.Code)
	jqCacheOrder []string
)

func (g *Client) applyListJQ(ctx context.Context, payload any, expression string) (any, error) {
	trimmedExpression := strings.TrimSpace(expression)
	if trimmedExpression == "" {
		return payload, nil
	}

	code, err := g.compileListJQCode(ctx, trimmedExpression)
	if err != nil {
		return nil, faults.NewValidationError("invalid list jq expression", err)
	}

	runCtx := ctx
	if runCtx == nil {
		runCtx = context.Background()
	}
	iterator := code.RunWithContext(runCtx, payload)
	results := make([]any, 0, 1)
	for {
		value, ok := iterator.Next()
		if !ok {
			break
		}
		if valueErr, isErr := value.(error); isErr {
			return nil, faults.NewValidationError("failed to evaluate list jq expression", valueErr)
		}
		results = append(results, value)
	}

	if len(results) == 0 {
		return []any{}, nil
	}
	if len(results) == 1 {
		return results[0], nil
	}
	return results, nil
}

func (g *Client) compileListJQCode(ctx context.Context, expression string) (*gojq.Code, error) {
	if !strings.Contains(expression, "resource(") {
		return cachedListJQCode(expression)
	}

	query, err := gojq.Parse(expression)
	if err != nil {
		return nil, err
	}
	return gojq.Compile(query, gojq.WithFunction("resource", 1, 1, g.listJQResourceFunction(ctx)))
}

func cachedListJQCode(expression string) (*gojq.Code, error) {
	jqCacheMu.Lock()
	if cached, ok := jqCacheMap[expression]; ok {
		jqCacheMu.Unlock()
		return cached, nil
	}
	jqCacheMu.Unlock()

	query, err := gojq.Parse(expression)
	if err != nil {
		return nil, err
	}
	code, err := gojq.Compile(query)
	if err != nil {
		return nil, err
	}

	jqCacheMu.Lock()
	if cached, ok := jqCacheMap[expression]; ok {
		jqCacheMu.Unlock()
		return cached, nil
	}
	if len(jqCacheOrder) >= maxJQCacheEntries {
		evict := jqCacheOrder[0]
		jqCacheOrder = jqCacheOrder[1:]
		delete(jqCacheMap, evict)
	}
	jqCacheMap[expression] = code
	jqCacheOrder = append(jqCacheOrder, expression)
	jqCacheMu.Unlock()

	return code, nil
}

func (g *Client) listJQResourceFunction(ctx context.Context) func(any, []any) any {
	cache := make(map[string]resource.Value)

	return func(_ any, args []any) any {
		logicalPath, err := parseListJQResourcePathArg(args)
		if err != nil {
			return err
		}

		if cached, exists := cache[logicalPath]; exists {
			return cached
		}

		resolved, err := g.resolveListJQResource(ctx, logicalPath)
		if err != nil {
			return err
		}

		cache[logicalPath] = resolved
		return resolved
	}
}

func parseListJQResourcePathArg(args []any) (string, error) {
	if len(args) != 1 {
		return "", fmt.Errorf("resource() expects exactly one path argument")
	}

	pathValue, ok := args[0].(string)
	if !ok {
		return "", fmt.Errorf("resource() path argument must be a string")
	}

	trimmed := strings.TrimSpace(pathValue)
	if trimmed == "" {
		return "", fmt.Errorf("resource() path argument must not be empty")
	}

	return trimmed, nil
}

func (g *Client) resolveListJQResource(ctx context.Context, logicalPath string) (resource.Value, error) {
	resolved, found, err := managedserverdomain.ResolveListJQResource(ctx, logicalPath)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, faults.NewValidationError("resource() requires list resolver context", nil)
	}
	return resource.Normalize(resolved)
}
