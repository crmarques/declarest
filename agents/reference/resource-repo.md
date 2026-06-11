# Resource Repository Layout and Safety

## Purpose
Define local repository semantics for resource/metadata on-disk layout, path safety, payload-file discovery, defaults-artifact layout, and Git/FS lifecycle and sync.

## Normative Rules

### Path safety
1. All paths MUST be normalized to logical absolute paths before IO.
2. Filesystem joins MUST reject traversal outside configured roots.
3. Segment `_` MUST be treated as the reserved metadata-namespace directory (see domain.md for `_` semantics).

### On-disk layout
4. Canonical resource payload MUST live at `<logical-path>/resource.<ext>`; this is the only canonical payload file per logical resource.
5. Resource/collection metadata sidecars MUST support both `metadata.yaml` and `metadata.json`, MUST prefer `metadata.yaml` when both exist, and SHOULD write `metadata.yaml` by default. Collection metadata lives at `<collection-path>/_/metadata.<yaml|json>`; resource metadata at `<logical-path>/metadata.<yaml|json>`.
6. Metadata-owned defaults artifacts MAY exist at selector-local deterministic names: collection scope `<collection-path>/_/defaults.<ext>` and `<collection-path>/_/defaults-<profile>.<ext>`; resource scope `<logical-path>/defaults.<ext>` and `<logical-path>/defaults-<profile>.<ext>`, where `<ext>` is `json|yaml|yml|properties`. Defaults SEMANTICS (merge/precedence/profiles/includes) are owned by metadata.md; this file only fixes their file layout and reserves their names.
7. Repository backends MUST reserve the entire `defaults` filename prefix for metadata-owned defaults artifacts so payload artifacts cannot collide with them. Backends MAY keep control artifacts under a repo-specific hidden directory; such directories MUST be excluded from payload/metadata discovery and from `tree`.

### Payload type resolution
8. New-file writes MUST resolve payload type explicitly: use metadata/file discovery first, then fall back to the repository default (`json`, `yaml`, `xml`, `hcl`, `ini`, `properties`, `text`, or `octet-stream`).
9. Opaque input files with unknown suffixes MUST preserve the provided suffix and treat the payload as octet-stream; `.bin` MUST be used only when no suffix or stronger payload hint is available.

### Payload discovery and isolation
10. Payload discovery MUST ignore metadata-owned defaults artifacts and MUST treat `resource.<ext>` as the only canonical payload file.
11. Repository reads and writes MUST operate on raw payload files only; effective-defaults merge/compaction belongs to metadata-aware orchestrator/CLI workflows, not to repository discovery.

### Save / delete / sidecar writes
12. Save operations MUST be atomic at file level.
13. When `resource save --secret` is selected, or metadata-driven whole-resource secret handling applies, the payload file MUST preserve the original descriptor-derived suffix and MUST contain only the exact root placeholder encoded for that payload type (for example raw `{{secret .}}` bytes for octet-stream). `{{secret .}}` key mapping is owned by secrets.md.
14. Resource delete MUST remove `resource.<ext>` overrides and MUST NOT remove metadata-owned defaults artifacts referenced by selector metadata.
15. Sidecar artifact writes MUST reject reserved sibling names (`resource.<ext>` or any name beginning with `defaults`) so they cannot overwrite canonical payloads or metadata-managed defaults.

### Listing and inspection policy
16. `list` MUST default to direct-children listing and MAY traverse descendants when `ListPolicy.Recursive=true`.
17. `delete` MUST default to removing only direct resources in a collection and MUST preserve subcollections unless `DeletePolicy.Recursive=true`.
18. `tree` MUST return deterministic, lexicographically sorted, repository-relative directory paths; MUST omit files; and MUST omit hidden control directories (for example `.git`) and reserved `_` metadata directories.

### Git/FS lifecycle and sync
19. Repository operations MUST be idempotent for repeated equivalent inputs.
20. Git-backed repositories MAY expose optional local commit/history; filesystem repositories MUST report history as unsupported.
21. Git-backed operations requiring local VCS state (status, history, commit, sync) MUST auto-initialize the local git repository when missing before running operation-specific logic.
22. `clean` MUST remove uncommitted tracked and untracked worktree changes for git repositories and MUST be a no-op for filesystem repositories.
23. Sync conflicts MUST surface typed conflict errors with remediation hints.
24. Push operations MUST never leak credentials in error output.
25. Git-backed repositories MAY configure authenticated webhook signaling via `spec.git.webhook` (`provider`, `secretRef`); receivers MUST verify provider-specific signatures/tokens before triggering reconcile. Receiver internals are defined in k8s-operator.md.

## Data Contracts
Manager method families (Go signatures owned by interfaces.md):
1. Resource IO: save/get/delete/list/move/exists.
2. Lifecycle: init/check/refresh/clean/reset.
3. Sync: push (with options)/status.
4. Optional VCS: commit/history.
5. Optional inspection: directory `tree`.

## Failure Modes
1. Traversal via relative segments -> rejected.
2. Multiple files matching `resource.*` under one logical path -> ambiguous payload error.
3. Unknown payload suffix without metadata/default guidance -> unresolved type error.
4. Push rejected due to remote divergence; or push on a freshly auto-initialized repo with no local HEAD/commit.
5. Sync requested with no remote configured -> `state: no_remote`.
6. History requested from a non-VCS (filesystem) backend -> not-supported, no mutation.
7. Webhook payload with invalid provider signature/token -> rejected.
8. Defaults artifact with unsupported type or non-object shape, or two same-role defaults artifacts matching one selector scope -> rejected.
9. Sidecar write to a reserved name (`resource.<ext>`, `defaults*`) -> rejected.

## Edge Cases
1. Alias change renames the payload directory while payload content is unchanged.
2. Simultaneous metadata and resource path updates in one operation.
3. Empty collection list with metadata present.
4. Reset requested with uncommitted local changes.
5. Concrete metadata `resource.format` resolves runtime behavior even when the on-disk suffix is unknown.
6. A child resource that compacts entirely to defaults still keeps an explicit (possibly empty) `resource.<ext>` so the logical resource remains present.
7. An explicit `null` override in `resource.<ext>` MUST win over a defaults value (merge invariant owned by metadata.md).
8. Push webhook for a branch not matching the configured branch is ignored without mutation.

## Examples
1. Save `/customers/acme` in a JSON context writes `/customers/acme/resource.json`; saving `/certificates/ca` as octet-stream with no existing file writes `/certificates/ca/resource.bin`.
2. Save `/projects/platform/secrets/private-key` from input `private.key` writes `resource.key` with octet-stream semantics (suffix preserved, not rewritten to `.bin`); adding `--secret` writes `resource.key` containing only `{{secret .}}` while the real bytes go to the secret store.
3. Set collection metadata for `/customers` writes `/customers/_/metadata.yaml`; when both `metadata.yaml` and `metadata.json` exist, reads resolve the YAML sidecar deterministically.
4. Alias change `acme` -> `acme-inc` moves payload from `/customers/acme/resource.*` to `/customers/acme-inc/resource.*`.
5. `repository status` with no remote returns `state: no_remote` (zero ahead/behind); against a base dir lacking `.git/` it auto-initializes git, then returns a deterministic status report.
6. `repository history` on a filesystem backend prints a deterministic not-supported message with no mutation; on git, `--path customers --grep fix --max-count 5` returns only local commits matching the combined filters.
7. `repository clean` on git discards tracked edits and removes untracked files/directories; on filesystem it succeeds without mutating files. `repository tree` returns directories like `admin/realms/acme/user-registry/AD PRD` and omits `.git/`, `_/`, and payload/metadata files.
8. With `/customers/_/metadata.yaml` plus `/customers/_/defaults.yaml` and `/customers/acme/resource.yaml`: repository `Get(/customers/acme)` returns the raw payload, payload discovery ignores `defaults*` (including a referenced `defaults-prod.properties`), and only orchestrator-backed reads merge defaults and compact saves back into `resource.yaml`. A valid authenticated push webhook updates webhook-receipt annotations and triggers immediate reconcile.
