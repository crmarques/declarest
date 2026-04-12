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
	"testing"
	"time"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func TestBuildRequestThrottleValidatesConfiguration(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		cfg  *config.HTTPRequestThrottling
	}{
		{
			name: "missing strategy",
			cfg:  &config.HTTPRequestThrottling{},
		},
		{
			name: "negative max concurrent",
			cfg: &config.HTTPRequestThrottling{
				MaxConcurrentRequests: -1,
			},
		},
		{
			name: "queue without concurrency",
			cfg: &config.HTTPRequestThrottling{
				QueueSize: 1,
			},
		},
		{
			name: "burst without rps",
			cfg: &config.HTTPRequestThrottling{
				MaxConcurrentRequests: 1,
				Burst:                 2,
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := buildRequestThrottle(tc.cfg)
			if !faults.IsCategory(err, faults.ValidationError) {
				t.Fatalf("expected validation error, got %v", err)
			}
		})
	}
}

func TestBuildRequestThrottleSharesGateByScopeKey(t *testing.T) {
	t.Parallel()

	cfg := &config.HTTPRequestThrottling{
		ScopeKey:              "default/shared",
		MaxConcurrentRequests: 1,
	}
	gateOne, err := buildRequestThrottle(cfg)
	if err != nil {
		t.Fatalf("buildRequestThrottle gateOne returned error: %v", err)
	}

	gateTwo, err := buildRequestThrottle(cfg)
	if err != nil {
		t.Fatalf("buildRequestThrottle gateTwo returned error: %v", err)
	}
	if gateOne != gateTwo {
		t.Fatal("expected shared gate for same scope key and config")
	}

	other, err := buildRequestThrottle(&config.HTTPRequestThrottling{
		ScopeKey:              "default/other",
		MaxConcurrentRequests: 1,
	})
	if err != nil {
		t.Fatalf("buildRequestThrottle other returned error: %v", err)
	}
	if other == gateOne {
		t.Fatal("expected different gate for different scope key")
	}
}

func TestRequestThrottleGateQueueRejectsWhenFull(t *testing.T) {
	t.Parallel()

	gate := newRequestThrottleGate(&config.HTTPRequestThrottling{
		MaxConcurrentRequests: 1,
		QueueSize:             1,
	})

	releaseOne, err := gate.acquire(context.Background())
	if err != nil {
		t.Fatalf("first acquire returned error: %v", err)
	}

	secondDone := make(chan struct{})
	var secondErr error
	var releaseTwo func()
	go func() {
		defer close(secondDone)
		releaseTwo, secondErr = gate.acquire(context.Background())
	}()

	deadline := time.Now().Add(2 * time.Second)
	for len(gate.queue) != 1 {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for queued request")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, err := gate.acquire(context.Background()); !faults.IsCategory(err, faults.ConflictError) {
		t.Fatalf("expected conflict error when queue is full, got %v", err)
	}

	releaseOne()
	<-secondDone
	if secondErr != nil {
		t.Fatalf("second acquire returned error after release: %v", secondErr)
	}
	if releaseTwo == nil {
		t.Fatal("expected second release function to be populated")
	}
	releaseTwo()
}
