# Orchestrator and Integration

## Purpose
Define orchestration behavior that coordinates repository, metadata, server, and secret operations for all user workflows.

## In Scope
1. High-level use-case orchestration.
2. Local/remote source-of-truth transitions.
3. Fallback and conflict handling strategy.
4. Deterministic apply/refresh/diff/list behavior.

## Out of Scope
1. Transport protocol internals.
2. Filesystem provider implementation internals.
3. CLI parsing details.

## Normative Rules
1. `orchestrator.Orchestrator` MUST be the only component that coordinates multiple managers in one workflow.
2. Workflows MUST declare mutation scope: local, remote, or both.
3. Apply-like operations MUST resolve metadata and identity before remote mutations.
4. Workflows that persist local state MUST use normalized `resource.Resource` and payload.
5. Idempotent repeated apply with unchanged desired state MUST produce no additional mutations.
6. Fallback behavior MUST be deterministic and bounded; no unbounded search loops.
7. Conflict conditions MUST return typed `ConflictError` with actionable context.
8. Compare/diff output MUST be stable for identical inputs.
9. Single-resource local workflows MUST attempt literal repository lookup first and then bounded collection fallback by metadata `idFromAttribute` when literal lookup returns `NotFound`.
10. Remote delete workflows SHOULD attempt literal delete first and MAY retry once with metadata-aware identity fallback after `NotFound`.
11. Remote read workflows SHOULD treat `NotFound` collection reads as empty collections only when repository structure hints or OpenAPI inference indicate the requested path is a collection endpoint, and they SHOULD preserve `NotFound` when a nested collection read fails because the parent resource is also `NotFound`.
12. Remote read metadata fallback MAY accept a single-candidate list result when metadata declares list `jq` filtering, but only when the requested logical path depth does not exceed the resolved selector/collection template depth; singleton fallback MUST NOT collapse explicit child identity segments and SHOULD then resolve to canonical remote identity for follow-up reads when possible.

## Data Contracts
Core orchestrator workflows:
1. Local read/write: get/save/list local resources.
2. Remote read/write: get/create/update/delete/list remote resources.
3. Orchestration workflows: apply, refresh, explain, diff, template.
4. Repository administration: init, refresh, push, reset, check, status.

Policy contract:
1. List workflows MUST accept explicit recursion policy and default to non-recursive behavior.

Resolution contract:
1. Input path -> metadata resolution -> `resource.Resource` identity resolution -> operation spec -> execution.
2. Optional secret masking/resolution performed at boundaries.

## Failure Modes
1. Metadata resolved but required remote identity missing.
2. Remote fetch/mutation succeeds but local persist fails in workflows that write repository state.
3. Remote fallback candidates are ambiguous.
4. Repository sync conflict blocks local persistence workflows after successful remote operations.
5. Local metadata-id fallback yields multiple candidates for one path segment.

## Edge Cases
1. Direct get path fails and alias-based fallback succeeds.
2. Alias changes between local and remote while remote ID remains stable.
3. Remote list returns duplicate alias candidates.
4. Dry-run explain differs from live apply due to dynamic template context drift.
5. Local path segment is an ID while repository uses metadata alias for stored logical paths.
6. Collection endpoint returns `404` when empty and must be interpreted as zero items rather than missing endpoint when collection hints are present and the parent resource exists.
7. Nested collection read returns `404` because the parent resource is missing and must remain `NotFound` instead of collapsing to `[]`.

## Examples
1. `Apply(/customers/acme)` resolves metadata, builds update path, resolves secrets, and performs a remote create/update mutation from repository desired state.
2. `Refresh(/customers)` lists remote collection, maps each item to deterministic alias paths, and writes local files.
3. `Diff(/customers/acme)` loads local and remote payloads, applies compare transforms, and returns deterministic operations.
4. `Apply(/admin/realms/master/clients/<uuid>)` resolves the local resource by metadata ID fallback when only alias-based repository paths exist.
5. `GetRemote(/admin/realms/master/organizations)` returns an empty list when the server responds `404` for an empty collection and collection hints confirm the path is a collection.
6. `GetRemote(/admin/realms/acme/organizations)` preserves `NotFound` when `/admin/realms/acme` is also missing, even if OpenAPI hints confirm `organizations` is a collection endpoint.
