# Reconciler and Integration

## Purpose
Define orchestration behavior that coordinates repository, metadata, server, and secret operations for all user workflows.

## In Scope
1. High-level use-case orchestration.
2. Local/remote source-of-truth transitions.
3. Fallback and conflict handling strategy.
4. Deterministic apply/refresh/diff/list behavior.

## Out of Scope
1. Transport protocol internals.
2. Filesystem adapter implementation internals.
3. CLI parsing details.

## Normative Rules
1. `Reconciler` MUST be the only component that coordinates multiple managers in one workflow.
2. Workflows MUST declare mutation scope: local, remote, or both.
3. Apply-like operations MUST resolve metadata and identity before remote mutations.
4. Local state persistence after remote mutation MUST use normalized `ResourceInfo` and payload.
5. Idempotent repeated apply with unchanged desired state MUST produce no additional mutations.
6. Fallback behavior MUST be deterministic and bounded; no unbounded search loops.
7. Conflict conditions MUST return typed `ConflictError` with actionable context.
8. Compare/diff output MUST be stable for identical inputs.

## Data Contracts
Core reconciler workflows:
1. Local read/write: get/save/list local resources.
2. Remote read/write: get/create/update/delete/list remote resources.
3. Reconciliation: apply, refresh, explain, diff, template.
4. Repository administration: init, refresh, push, reset, check.

Resolution contract:
1. Input path -> metadata resolution -> `ResourceInfo` identity resolution -> operation spec -> execution.
2. Optional secret masking/resolution performed at boundaries.

## Failure Modes
1. Metadata resolved but required remote identity missing.
2. Remote mutation succeeds but local persist fails.
3. Remote fallback candidates are ambiguous.
4. Repository sync conflict blocks persistence after successful mutation.

## Edge Cases
1. Direct get path fails and alias-based fallback succeeds.
2. Alias changes between local and remote while remote ID remains stable.
3. Remote list returns duplicate alias candidates.
4. Dry-run explain differs from live apply due to dynamic template context drift.

## Examples
1. `Apply(/customers/acme)` resolves metadata, builds update path, resolves secrets, performs remote update, then persists normalized local payload.
2. `Refresh(/customers)` lists remote collection, maps each item to deterministic alias paths, and writes local files.
3. `Diff(/customers/acme)` loads local and remote payloads, applies compare transforms, and returns deterministic operations.
