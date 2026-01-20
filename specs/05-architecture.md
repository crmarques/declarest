# Architecture and Ownership

## Stable contracts
Do not change interface shapes without explicit design approval.

## Layers (MUST remain)
- `cli/cmd`: Cobra commands; parse args/flags, print output; calls only `Reconciler`.
- `reconciler`: orchestrates compare/diff/sync using metadata + managers.
- `managedserver`: resource server contracts + HTTP implementation.
- `repository`: repo contracts + Git/FS implementations.
- `metadata`: metadata loading, layering, access.
- `secrets`: secret loading, access, saving.
- `context`: context discovery/config wiring (ContextManager).
- `resource`: JSON helpers, patching, diffing.

## Hard boundaries (MUST)
- Repository I/O only via `ResourceRepositoryManager`.
- Server I/O only via `ResourceServerManager`.
- Metadata lookups only via `MetadataProvider`.
- Context lifecycle only via `ContextManager`.
- Secret I/O only via `SecretsManager`.
- All orchestration only via `Reconciler`.
