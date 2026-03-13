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
4. Resource payload type MUST be explicit for new-file writes, using metadata/file discovery before falling back to the repository default (`json`, `yaml`, `xml`, `hcl`, `ini`, `properties`, `text`, or `octet-stream`); opaque input files with unknown suffixes MUST preserve the provided suffix and use `.bin` only when no suffix or stronger payload hint is available.
5. Save operations MUST be atomic at file level.
6. Repository sync conflicts MUST surface typed conflict errors with remediation hints.
7. Push operations MUST never leak credentials in error output.
8. Repository operations MUST be idempotent for repeated equivalent inputs.
9. Git-backed repositories MAY expose optional local commit/history capabilities; filesystem repositories MUST report history as unsupported.
10. Git-backed repository operations that require a local VCS repository state (for example status, history, commit, sync operations) MUST initialize the local git repository automatically when it is missing before continuing operation-specific logic.
11. `clean` MUST remove uncommitted tracked and untracked worktree changes for git repositories and MUST be a no-op for filesystem repositories.
12. Git-backed repositories MAY configure authenticated webhook signaling; repository webhook receivers MUST verify provider-specific signatures/tokens before triggering reconcile (detailed receiver behavior is defined in `agents/reference/k8s-operator.md`).
13. Resource and collection metadata sidecars MUST support `metadata.yaml` and `metadata.json`, MUST prefer `metadata.yaml` when both exist, and SHOULD write `metadata.yaml` by default.
14. When `resource save --secret` is selected or metadata-driven whole-resource secret handling applies, the repository payload file MUST preserve the original descriptor-derived suffix and MUST contain only the exact root placeholder encoded for that payload type (for example raw `{{secret .}}` bytes for octet-stream files).
15. One logical collection MAY also include one optional metadata-root defaults sidecar at `<collection-path>/_/defaults.<ext>`; when present, repository reads for direct child resources under that collection MUST treat it as part of each logical resource instead of as a separate resource path.
16. Resource directories MUST NOT contain per-resource defaults files at `<logical-path>/defaults.<ext>`; encountering that legacy layout MUST fail repository-backed workflows with `ValidationError` and remediation toward `<collection-path>/_/defaults.<ext>`.
17. `defaults.<ext>` and `resource.<ext>` MUST resolve to compatible merge-capable payload types before workflow use; JSON and YAML defaults MUST interoperate with JSON or YAML resource payloads, while unsupported or incompatible combinations MUST fail with a typed validation or conflict error.
18. Defaults sidecars MUST be supported only for merge-capable object payload codecs (`json`, `yaml`, `ini`, `properties`); array-root payloads, scalar-root payloads, `xml`, `hcl`, generic text, `octet-stream`, and whole-resource-secret payloads MUST fail defaults-sidecar validation instead of being merged implicitly.
19. Repository reads for resources with both collection defaults and `resource.<ext>` MUST expose the effective payload produced by deep-merging object fields, replacing arrays, and letting explicit `resource.<ext>` values override defaults deterministically.
20. Repository writes for resources with resolved `defaults.<ext>` MUST compact the effective payload against the defaults before writing `resource.<ext>` so unchanged defaulted fields do not get flattened back into the override file; when a collection-level defaults sidecar makes every field defaulted, repository persistence MUST still keep an explicit empty override file so the logical resource remains present.
21. Resource delete workflows MUST remove `resource.<ext>` overrides but MUST NOT remove collection-level metadata defaults sidecars.
22. Repository implementations that expose raw defaults-sidecar editing MUST provide a separate read/write capability for metadata-root `defaults.<ext>` so CLI workflows can inspect or edit defaults without flattening them into effective resource payloads, MUST preserve an existing defaults-sidecar codec when rewriting that sidecar, MUST otherwise prefer the target collection's merge-capable resource payload type when creating a new defaults sidecar, and SHOULD fall back to JSON or YAML only when no merge-capable resource payload type is resolved.
23. Sidecar artifact writes MUST reject reserved sibling payload names such as `resource.<ext>` or `defaults.<ext>` so artifact workflows cannot overwrite canonical payload/defaults files.

## Data Contracts
Layout contract:
1. Canonical resource payload at `<logical-path>/resource.<ext>`.
2. Optional merge-capable object defaults sidecar at `<collection-path>/_/defaults.<ext>`.
3. Collection metadata at `<collection-path>/_/metadata.yaml` by default, with `metadata.json` also accepted.
4. Resource metadata at `<logical-path>/metadata.yaml` by default, with `metadata.json` also accepted.
5. Optional repository control artifacts under repo-specific hidden directory.
6. Optional git webhook contract under `spec.git.webhook` (`provider`, `secretRef`) for operator-triggered refresh signaling.

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
2. Multiple payload files matching `resource.*` under one logical path.
3. Unknown payload suffix without metadata/default type guidance.
4. Push rejected due to remote divergence.
5. Missing remote configuration for sync operation.
6. History requested from a repository backend that does not support local VCS history.
7. Push requested on a freshly auto-initialized git repo without a local HEAD/commit.
8. Webhook payload rejected due to invalid provider signature/token.
9. A repository still contains a legacy per-resource defaults file at `<logical-path>/defaults.<ext>`.
10. `defaults.<ext>` uses an incompatible or unsupported payload type/shape for the effective resource payload.
11. Multiple defaults sidecars match one logical collection scope.
12. An artifact write attempts to use a reserved payload sidecar name such as `defaults.yaml`.

## Edge Cases
1. Rename required after alias change while keeping payload unchanged.
2. Simultaneous metadata and resource path updates in one operation.
3. Empty collection list with metadata present.
4. Reset requested with uncommitted local changes.
5. Existing payload suffix is unknown but metadata `payloadType` still resolves runtime behavior.
6. Opaque file input uses an unknown suffix (for example `.key`) and persistence keeps `resource.key` with octet-stream semantics instead of rewriting to `resource.bin`.
7. Whole-resource secret saves keep the original payload suffix while replacing the repository payload with an exact root placeholder.
8. Auto-commit-enabled CLI repository mutations run while unrelated git worktree changes are present.
9. First repo interaction runs against an existing repository base directory that has resource files but no `.git/` directory yet.
10. Clean requested on a git repo with both tracked edits and untracked files/directories.
11. Clean requested on a filesystem repository context.
12. Valid push webhook arrives for a branch that does not match the configured repository branch and is ignored without mutation.
13. Both `metadata.yaml` and `metadata.json` exist for one selector path; repository-backed metadata resolution uses the YAML sidecar deterministically.
14. `customers/_/defaults.yaml` declares an object field that `customers/acme/resource.yaml` explicitly sets to `null`; the effective payload MUST keep the explicit `null` override instead of restoring the default.
15. A collection has `_/defaults.yaml` and a child resource compacts entirely to defaults; repository writes preserve logical resource presence with an explicit empty `resource.<ext>` override file rather than deleting the resource.
16. A repository still contains `customers/acme/defaults.yaml` from an older layout; repository-backed defaults workflows MUST fail with `ValidationError` and direct the user to move that file under `customers/_/defaults.yaml`.

## Examples
1. Save `/customers/acme` in JSON context writes `/customers/acme/resource.json`.
2. Save `/projects/platform/readme` as plain text writes `/projects/platform/readme/resource.txt`.
3. Save `/certificates/ca` as octet-stream without an existing file writes `/certificates/ca/resource.bin`.
4. Save `/projects/platform/secrets/private-key` from input file `private.key` writes `/projects/platform/secrets/private-key/resource.key` and still treats the payload as octet-stream.
5. Save `/projects/platform/secrets/private-key --secret` from input file `private.key` writes `/projects/platform/secrets/private-key/resource.key` containing only `{{secret .}}` while the original payload bytes live in the secret store.
6. Set collection metadata for `/customers` writes `/customers/_/metadata.yaml`.
7. Alias change from `acme` to `acme-inc` moves payload from `/customers/acme/resource.*` to `/customers/acme-inc/resource.*`.
8. `status` on a repository without remote configuration returns `state: no_remote` with zero ahead/behind counts.
9. `repository history` on a filesystem repository prints a deterministic not-supported message and performs no repository mutation.
10. `repository history --path customers --grep fix --max-count 5` returns only local commits matching the combined filters when the backend is git.
11. `repository status` on a git context with an existing base directory but no `.git/` auto-initializes the local git repository and then returns a deterministic sync status report.
12. `repository clean` on a git repository discards tracked worktree edits and removes untracked files/directories.
13. `repository clean` on a filesystem repository succeeds without mutating repository files.
14. `repository tree` returns directories like `admin/realms/acme/user-registry/AD PRD` and omits `.git/`, `_/`, and payload/metadata files.
15. A valid authenticated git push webhook updates repository webhook receipt annotations and triggers immediate repository reconcile without waiting for the next poll interval.
16. When `/customers/_/metadata.yaml` and `/customers/_/metadata.json` both exist, metadata reads resolve `/customers/_/metadata.yaml` deterministically.
17. `/customers/_/defaults.yaml` plus `/customers/acme/resource.yaml` behaves as one logical resource; `Get(/customers/acme)` returns the merged object and `Save(/customers/acme)` writes only fields that differ from `defaults.yaml` back into `resource.yaml`.
18. `/customers/_/defaults.properties` plus `/customers/acme/resource.properties` merges and compacts Java-properties key/value objects just like JSON/YAML resources.
