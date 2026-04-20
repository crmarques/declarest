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

package orchestrator

import (
	"context"

	"github.com/crmarques/declarest/repository"
)

// DeletePolicy is an alias for repository.DeletePolicy — the recursive-scope
// directive is the same at both boundaries.
type DeletePolicy = repository.DeletePolicy

// ListPolicy is an alias for repository.ListPolicy.
type ListPolicy = repository.ListPolicy

// ConflictCheck summarizes everything the orchestrator knows at apply time
// that a higher-level arbitrator (e.g. SyncPolicy checking the CRDGenerator
// conflict index) may need to veto the mutation.
type ConflictCheck struct {
	LogicalPath    string
	CollectionPath string
	RemoteID       string
}

// ConflictChecker is an optional tier-2 arbitrator consulted by the
// orchestrator just before it executes a remote create/update. Returning
// true skips the mutation and leaves remote state untouched — the caller is
// expected to have already raised the corresponding observability signal
// (event, metric, condition).
type ConflictChecker func(ctx context.Context, check ConflictCheck) (skip bool, reason string)

type ApplyPolicy struct {
	Force    bool
	Conflict ConflictChecker
}
