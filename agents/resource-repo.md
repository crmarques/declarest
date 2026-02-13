# Resource Repository Layout and Safety

## Purpose
Define local repository semantics for resource persistence, metadata storage, path safety, and optional Git synchronization.

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

## Data Contracts
Layout contract:
1. Resource payload at `<logical-path>.<ext>`.
2. Collection metadata at `<collection-path>/_/metadata.<ext>`.
3. Resource metadata at `<logical-path>/metadata.<ext>`.
4. Optional repository control artifacts under repo-specific hidden directory.

Manager method families:
1. Resource IO: save/get/delete/list/move/exists.
2. Repository lifecycle: init/check/refresh/reset.
3. Sync: push/force-push/sync-status.

## Failure Modes
1. Traversal attempt using relative path segments.
2. Resource and metadata extension mismatch for context format.
3. Push rejected due to remote divergence.
4. Missing remote configuration for sync operation.

## Edge Cases
1. Rename required after alias change while keeping payload unchanged.
2. Simultaneous metadata and resource path updates in one operation.
3. Empty collection list with metadata present.
4. Reset requested with uncommitted local changes.

## Examples
1. Save `/customers/acme` in JSON context writes `/customers/acme.json`.
2. Set collection metadata for `/customers` writes `/customers/_/metadata.json`.
3. Alias change from `acme` to `acme-inc` triggers move from `/customers/acme.*` to `/customers/acme-inc.*`.
