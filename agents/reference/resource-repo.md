# Resource Repository Layout and Safety

## Purpose
Define local repository semantics for resource persistence, metadata storage, path safety, and optional Git synchronization/history capabilities.

## In Scope
1. Logical path normalization.
2. Resource and metadata on-disk layout.
3. Git/FS repository lifecycle and sync behavior.
4. Safety invariants for filesystem operations.

## Out of Scope
1. Remote HTTP execution.
2. CLI user interaction details.
3. Secret backend protocols.

## Normative Rules
1. All paths MUST be normalized logical absolute paths before IO.
2. Filesystem joins MUST reject traversal outside configured roots.
3. Reserved segment `_` MUST be treated as metadata namespace.
4. Resource data format MUST be explicit (`json` or `yaml`) per context.
5. Save operations MUST be atomic at file level.
6. Repository sync conflicts MUST surface typed conflict errors with remediation hints.
7. Push operations MUST never leak credentials in error output.
8. Repository operations MUST be idempotent for repeated equivalent inputs.
9. Git-backed repositories MAY expose optional local commit/history capabilities; filesystem repositories MUST report history as unsupported.
10. Git-backed repository operations that require a local VCS repository state (for example status, history, commit, sync operations) MUST initialize the local git repository automatically when it is missing before continuing operation-specific logic.
11. `clean` MUST remove uncommitted tracked and untracked worktree changes for git repositories and MUST be a no-op for filesystem repositories.
12. Git-backed repositories MAY configure authenticated webhook signaling; repository webhook receivers MUST verify provider-specific signatures/tokens before triggering reconcile (detailed receiver behavior is defined in `agents/reference/k8s-operator.md`).

## Data Contracts
Layout contract:
1. Canonical resource payload at `<logical-path>/resource.<ext>`.
2. Collection metadata at `<collection-path>/_/metadata.<ext>`.
3. Resource metadata at `<logical-path>/metadata.<ext>`.
4. Optional repository control artifacts under repo-specific hidden directory.
5. Optional git webhook contract under `spec.git.webhook` (`provider`, `secretRef`) for operator-triggered refresh signaling.

Manager method families:
1. Resource IO: save/get/delete/list/move/exists.
2. Repository lifecycle: init/check/refresh/clean/reset.
3. Sync: push (with options)/status.
4. Optional VCS capabilities: commit/history.
5. Optional inspection capabilities: directory tree (`tree`).

Policy contracts:
1. `list` MUST default to direct-children listing and MAY traverse descendants when `ListPolicy.Recursive=true`.
2. `delete` MUST default to removing only direct resources in a collection and MUST preserve subcollections unless `DeletePolicy.Recursive=true`.
3. Optional directory-tree inspection MUST return deterministic lexicographically sorted repository-relative directory paths, omit files, omit hidden control directories (for example `.git`), and omit reserved metadata namespace directories named `_`.

## Failure Modes
1. Traversal attempt using relative path segments.
2. Resource and metadata extension mismatch for context format.
3. Push rejected due to remote divergence.
4. Missing remote configuration for sync operation.
5. History requested from a repository backend that does not support local VCS history.
6. Push requested on a freshly auto-initialized git repo without a local HEAD/commit.
7. Webhook payload rejected due to invalid provider signature/token.

## Edge Cases
1. Rename required after alias change while keeping payload unchanged.
2. Simultaneous metadata and resource path updates in one operation.
3. Empty collection list with metadata present.
4. Reset requested with uncommitted local changes.
5. Auto-commit-enabled CLI repository mutations run while unrelated git worktree changes are present.
6. First repo interaction runs against an existing repository base directory that has resource files but no `.git/` directory yet.
7. Clean requested on a git repo with both tracked edits and untracked files/directories.
8. Clean requested on a filesystem repository context.
9. Valid push webhook arrives for a branch that does not match the configured repository branch and is ignored without mutation.

## Examples
1. Save `/customers/acme` in JSON context writes `/customers/acme/resource.json`.
2. Set collection metadata for `/customers` writes `/customers/_/metadata.json`.
3. Alias change from `acme` to `acme-inc` moves payload from `/customers/acme/resource.*` to `/customers/acme-inc/resource.*`.
4. `status` on a repository without remote configuration returns `state: no_remote` with zero ahead/behind counts.
5. `repository history` on a filesystem repository prints a deterministic not-supported message and performs no repository mutation.
6. `repository history --path customers --grep fix --max-count 5` returns only local commits matching the combined filters when the backend is git.
7. `repository status` on a git context with an existing base directory but no `.git/` auto-initializes the local git repository and then returns a deterministic sync status report.
8. `repository clean` on a git repository discards tracked worktree edits and removes untracked files/directories.
9. `repository clean` on a filesystem repository succeeds without mutating repository files.
10. `repository tree` returns directories like `admin/realms/acme/user-registry/AD PRD` and omits `.git/`, `_/`, and payload/metadata files.
11. A valid authenticated git push webhook updates repository webhook receipt annotations and triggers immediate repository reconcile without waiting for the next poll interval.
