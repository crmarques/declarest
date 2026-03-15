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
	"net/http"
	"sync"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"golang.org/x/time/rate"
)

type requestThrottleGate struct {
	limiter  *rate.Limiter
	inFlight chan struct{}
	queue    chan struct{}
}

var (
	sharedRequestThrottleMu sync.Mutex
	sharedRequestThrottles  = map[string]*requestThrottleGate{}
)

func buildRequestThrottle(cfg *config.HTTPRequestThrottling) (*requestThrottleGate, error) {
	if cfg == nil {
		return nil, nil
	}
	if cfg.MaxConcurrentRequests <= 0 && cfg.RequestsPerSecond <= 0 {
		return nil, faults.NewValidationError(
			"managed-server.http.request-throttling must define at least one of max-concurrent-requests or requests-per-second",
			nil,
		)
	}
	if cfg.MaxConcurrentRequests < 0 {
		return nil, faults.NewValidationError("managed-server.http.request-throttling.max-concurrent-requests must be greater than zero when set", nil)
	}
	if cfg.QueueSize < 0 {
		return nil, faults.NewValidationError("managed-server.http.request-throttling.queue-size must be greater than or equal to zero", nil)
	}
	if cfg.QueueSize > 0 && cfg.MaxConcurrentRequests <= 0 {
		return nil, faults.NewValidationError("managed-server.http.request-throttling.queue-size requires max-concurrent-requests", nil)
	}
	if cfg.RequestsPerSecond < 0 {
		return nil, faults.NewValidationError("managed-server.http.request-throttling.requests-per-second must be greater than zero when set", nil)
	}
	if cfg.Burst < 0 {
		return nil, faults.NewValidationError("managed-server.http.request-throttling.burst must be greater than zero when set", nil)
	}
	if cfg.Burst > 0 && cfg.RequestsPerSecond <= 0 {
		return nil, faults.NewValidationError("managed-server.http.request-throttling.burst requires requests-per-second", nil)
	}

	key := requestThrottleKey(cfg)
	if key == "" {
		return newRequestThrottleGate(cfg), nil
	}

	sharedRequestThrottleMu.Lock()
	defer sharedRequestThrottleMu.Unlock()

	if existing, ok := sharedRequestThrottles[key]; ok {
		return existing, nil
	}

	gate := newRequestThrottleGate(cfg)
	sharedRequestThrottles[key] = gate
	return gate, nil
}

func requestThrottleKey(cfg *config.HTTPRequestThrottling) string {
	if cfg == nil {
		return ""
	}
	scopeKey := cfg.ScopeKey
	if scopeKey == "" {
		return ""
	}
	return fmt.Sprintf(
		"%s|max=%d|queue=%d|rps=%f|burst=%d",
		scopeKey,
		cfg.MaxConcurrentRequests,
		cfg.QueueSize,
		cfg.RequestsPerSecond,
		cfg.Burst,
	)
}

func newRequestThrottleGate(cfg *config.HTTPRequestThrottling) *requestThrottleGate {
	gate := &requestThrottleGate{}
	if cfg == nil {
		return gate
	}
	if cfg.MaxConcurrentRequests > 0 {
		gate.inFlight = make(chan struct{}, cfg.MaxConcurrentRequests)
	}
	if cfg.QueueSize > 0 {
		gate.queue = make(chan struct{}, cfg.QueueSize)
	}
	if cfg.RequestsPerSecond > 0 {
		burst := cfg.Burst
		if burst <= 0 {
			burst = int(cfg.RequestsPerSecond)
			if burst < 1 {
				burst = 1
			}
		}
		gate.limiter = rate.NewLimiter(rate.Limit(cfg.RequestsPerSecond), burst)
	}
	return gate
}

func (g *requestThrottleGate) execute(
	ctx context.Context,
	invoke func() (*http.Response, error),
) (*http.Response, error) {
	release, err := g.acquire(ctx)
	if err != nil {
		return nil, err
	}
	defer release()
	if g.limiter != nil {
		if waitErr := g.limiter.Wait(ctx); waitErr != nil {
			return nil, faults.NewTypedError(faults.TransportError, "managed-server request throttling wait failed", waitErr)
		}
	}
	return invoke()
}

func (g *requestThrottleGate) acquire(ctx context.Context) (func(), error) {
	if g == nil {
		return func() {}, nil
	}

	queued := false
	if g.queue != nil {
		select {
		case g.queue <- struct{}{}:
			queued = true
		default:
			return nil, faults.NewConflictError("managed-server request queue is full", nil)
		}
	}

	if g.inFlight != nil {
		select {
		case g.inFlight <- struct{}{}:
			if queued {
				<-g.queue
			}
			return func() {
				<-g.inFlight
			}, nil
		case <-ctx.Done():
			if queued {
				<-g.queue
			}
			return nil, faults.NewTypedError(faults.TransportError, "managed-server request throttling wait canceled", ctx.Err())
		}
	}

	if queued {
		<-g.queue
	}
	return func() {}, nil
}
