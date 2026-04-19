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

package http

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/itchyny/gojq"

	"github.com/crmarques/declarest/faults"
	managedservicedomain "github.com/crmarques/declarest/managedservice"
	"github.com/crmarques/declarest/resource"
)

const maxJQCacheEntries = 128

var (
	jqCacheMu    sync.Mutex
	jqCacheMap   = make(map[string]*gojq.Code)
	jqCacheOrder []string
)

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
	resolved, found, err := managedservicedomain.ResolveListJQResource(ctx, logicalPath)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, faults.Invalid("resource() requires list resolver context", nil)
	}
	return resource.Normalize(resolved)
}
