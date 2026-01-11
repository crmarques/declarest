# Architecture and Ownership

## Stable contracts
Do not change interface shapes without explicit design approval.

## Layers (MUST remain)
- `cli/cmd`: Cobra commands; parse args/flags, print output; calls only `Reconciler`.
- `internal/reconciler`: orchestrates compare/diff/sync using metadata + managers.
- `internal/managedserver`: resource server contracts + HTTP implementation.
- `internal/repository`: repo contracts + Git/FS implementations.
- `internal/metadata`: metadata loading, layering, access.
- `internal/context`: context discovery/config wiring (ContextManager).
- `internal/resource`: JSON helpers, patching, diffing.

## Hard boundaries (MUST)
- Repository I/O only via `ResourceRepositoryManager`.
- Server I/O only via `ResourceServerManager`.
- Metadata lookups only via `MetadataProvider`.
- Context lifecycle only via `ContextManager`.
- All orchestration only via `Reconciler`.
