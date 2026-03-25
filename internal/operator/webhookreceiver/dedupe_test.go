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
	"testing"
	"time"
)

func TestDedupeCache(t *testing.T) {
	cache := NewDedupeCache(100 * time.Millisecond)

	// Empty delivery ID should never be a duplicate.
	if cache.IsDuplicate("") {
		t.Error("empty delivery ID should not be duplicate")
	}

	// First time should not be duplicate.
	if cache.IsDuplicate("delivery-1") {
		t.Error("first delivery should not be duplicate")
	}

	// Second time should be duplicate.
	if !cache.IsDuplicate("delivery-1") {
		t.Error("repeated delivery should be duplicate")
	}

	// Different delivery should not be duplicate.
	if cache.IsDuplicate("delivery-2") {
		t.Error("different delivery should not be duplicate")
	}

	// After TTL, entry should expire.
	time.Sleep(150 * time.Millisecond)
	if cache.IsDuplicate("delivery-1") {
		t.Error("expired delivery should not be duplicate")
	}
}
