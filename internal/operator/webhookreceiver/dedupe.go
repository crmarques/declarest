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

package webhookreceiver

import (
	"sync"
	"time"
)

// DedupeCache prevents duplicate webhook deliveries from triggering
// repeated reconciliation within a TTL window.
type DedupeCache struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
}

// NewDedupeCache creates a cache with the given TTL for delivery deduplication.
func NewDedupeCache(ttl time.Duration) *DedupeCache {
	return &DedupeCache{
		entries: make(map[string]time.Time),
		ttl:     ttl,
	}
}

// IsDuplicate returns true if the delivery ID has been seen within the TTL window.
// If not a duplicate, it records the delivery ID.
func (c *DedupeCache) IsDuplicate(deliveryID string) bool {
	if deliveryID == "" {
		return false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// Evict expired entries opportunistically.
	for k, v := range c.entries {
		if now.Sub(v) > c.ttl {
			delete(c.entries, k)
		}
	}

	if _, exists := c.entries[deliveryID]; exists {
		return true
	}

	c.entries[deliveryID] = now
	return false
}
