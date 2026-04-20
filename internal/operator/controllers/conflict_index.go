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

package controllers

import (
	"path"
	"strings"
	"sync"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
)

// ConflictSource represents who "owns" a remote resource in the shared
// CRDGenerator/SyncPolicy arbitration.
type ConflictSource struct {
	// CRDGenerator coordinates (namespace/name) that owns the entry.
	CRDGeneratorNamespace string
	CRDGeneratorName      string
	// Kind of the generated CR that registered this entry.
	GeneratedKind string
	// LogicalPath of the registered CR, used for event/log messages.
	LogicalPath string
	// RemoteID resolved during reconcile, populated only by the tier-2
	// registration. Empty when only the tier-1 name is known.
	RemoteID string
}

// conflictKey narrows the lookup to the three coordinates that define a
// conflict target on the remote: the managed service + the collection + the
// logical path (tier 1) OR + the resolved remote ID (tier 2).
type conflictKey struct {
	ManagedServiceNamespace string
	ManagedServiceName      string
	CollectionPath          string
	Identifier              string // logicalPath for tier 1, remoteID for tier 2
}

// ConflictIndex arbitrates between CRDGenerator-owned and SyncPolicy-owned
// resources. CRDGenerator registers the resources it manages; SyncPolicy
// consults the index before apply/prune and skips any entry that is owned by
// a CRDGenerator. The index is process-local — it exists to shortcut the
// race between the two reconcilers in the same operator pod and intentionally
// does NOT replace the authoritative Kubernetes state.
type ConflictIndex struct {
	mu    sync.RWMutex
	tier1 map[conflictKey]ConflictSource // keyed on logical path
	tier2 map[conflictKey]ConflictSource // keyed on resolved remote ID
}

// NewConflictIndex returns an empty index.
func NewConflictIndex() *ConflictIndex {
	return &ConflictIndex{
		tier1: make(map[conflictKey]ConflictSource),
		tier2: make(map[conflictKey]ConflictSource),
	}
}

// Register records ownership of the resource identified by the CRDGenerator
// coordinates + managed service binding. Passing a non-empty remoteID records
// a tier-2 entry in addition to the tier-1 entry.
func (c *ConflictIndex) Register(
	managedServiceNamespace string,
	managedServiceName string,
	collectionPath string,
	logicalPath string,
	remoteID string,
	source ConflictSource,
) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	tier1Key := conflictKey{
		ManagedServiceNamespace: strings.TrimSpace(managedServiceNamespace),
		ManagedServiceName:      strings.TrimSpace(managedServiceName),
		CollectionPath:          normalizeCollectionPath(collectionPath),
		Identifier:              normalizeLogicalPath(logicalPath),
	}
	c.tier1[tier1Key] = source

	if remoteID = strings.TrimSpace(remoteID); remoteID != "" {
		tier2Key := tier1Key
		tier2Key.Identifier = remoteID
		c.tier2[tier2Key] = source
	}
}

// Unregister removes all entries that belong to the given CRDGenerator,
// optionally scoped to a single generated CR's logical path + remote ID pair.
// If logicalPath is empty, every entry owned by the generator is cleared.
func (c *ConflictIndex) Unregister(
	crdGeneratorNamespace string,
	crdGeneratorName string,
	logicalPath string,
) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	normalizedPath := normalizeLogicalPath(logicalPath)
	purge := func(bucket map[conflictKey]ConflictSource) {
		for key, source := range bucket {
			if source.CRDGeneratorNamespace != crdGeneratorNamespace ||
				source.CRDGeneratorName != crdGeneratorName {
				continue
			}
			if normalizedPath != "" && source.LogicalPath != normalizedPath {
				continue
			}
			delete(bucket, key)
		}
	}
	purge(c.tier1)
	purge(c.tier2)
}

// Lookup reports whether a given (managedService, collectionPath, identifier)
// triple is owned by a CRDGenerator. Callers pass the logical path for tier 1
// checks, or the resolved remote ID for tier 2 checks.
func (c *ConflictIndex) Lookup(
	managedServiceNamespace string,
	managedServiceName string,
	collectionPath string,
	identifier string,
	tier ConflictTier,
) (ConflictSource, bool) {
	if c == nil {
		return ConflictSource{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	key := conflictKey{
		ManagedServiceNamespace: strings.TrimSpace(managedServiceNamespace),
		ManagedServiceName:      strings.TrimSpace(managedServiceName),
		CollectionPath:          normalizeCollectionPath(collectionPath),
	}
	switch tier {
	case ConflictTierName:
		key.Identifier = normalizeLogicalPath(identifier)
		src, ok := c.tier1[key]
		return src, ok
	case ConflictTierRemoteID:
		key.Identifier = strings.TrimSpace(identifier)
		src, ok := c.tier2[key]
		return src, ok
	}
	return ConflictSource{}, false
}

// ConflictTier differentiates the two registration levels.
type ConflictTier string

const (
	ConflictTierName     ConflictTier = "name"
	ConflictTierRemoteID ConflictTier = "remoteID"
)

// CollectionPathsForGenerator returns every (managedService, collectionPath)
// tuple that a specific CRDGenerator has registered. Consumed by SyncPolicy's
// overlap validation to surface static conflicts before runtime apply.
func (c *ConflictIndex) CollectionPathsForGenerator(namespace, name string) []string {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	seen := make(map[string]struct{})
	var out []string
	for key, source := range c.tier1 {
		if source.CRDGeneratorNamespace != namespace || source.CRDGeneratorName != name {
			continue
		}
		if _, exists := seen[key.CollectionPath]; exists {
			continue
		}
		seen[key.CollectionPath] = struct{}{}
		out = append(out, key.CollectionPath)
	}
	return out
}

func normalizeCollectionPath(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return path.Clean(value)
}

func normalizeLogicalPath(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	normalized := declarestv1alpha1.NormalizeOverlapPath(value)
	if normalized != "" {
		return normalized
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return path.Clean(value)
}
