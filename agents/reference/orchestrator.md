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
4. Apply workflows MUST read remote state first, create only on `NotFound`, and otherwise compare desired-vs-remote using metadata compare directives before deciding whether to update.
5. Apply workflows MUST skip update when compare output has no drift unless explicitly forced.
6. Workflows that persist local state MUST use normalized `resource.Resource` and payload.
7. Idempotent repeated apply with unchanged desired state MUST produce no additional mutations unless forced.
8. Fallback behavior MUST be deterministic and bounded; no unbounded search loops.
9. Conflict conditions MUST return typed `ConflictError` with actionable context.
10. Compare/diff output MUST be stable for identical inputs.
11. Binary payload comparison MUST be whole-payload only: identical bytes mean no drift, and differing bytes MUST produce one deterministic root-level replace diff entry.
12. Single-resource local workflows MUST attempt literal repository lookup first and then bounded collection fallback by metadata `resource.id`, using reverse matching only when the identity template is a simple single-pointer expression, when literal lookup returns `NotFound`.
13. Remote delete workflows SHOULD attempt literal delete first and MAY retry once with metadata-aware identity fallback after `NotFound`.
14. Remote read workflows SHOULD treat `NotFound` collection reads as empty collections only when repository structure hints or OpenAPI inference indicate the requested path is a collection endpoint, and they SHOULD preserve `NotFound` when a nested collection read fails because the parent resource is also `NotFound`.
15. Remote read metadata fallback MAY accept a single-candidate list result when metadata declares list `jq` filtering, but only when the requested logical path depth does not exceed the resolved selector/collection template depth; singleton fallback MUST NOT collapse explicit child identity segments and SHOULD then resolve to canonical remote identity for follow-up reads when possible.
16. Repository-backed local read, diff, template, and mutation-preparation workflows MUST use the effective local desired payload after metadata `resource.defaults` resolution and repository artifact expansion, before identity resolution, payload validation, secret resolution, compare transforms, or remote HTTP execution.
17. Repository-backed write workflows MUST preserve metadata-managed defaults layout by compacting effective desired payloads against the resolved metadata defaults object before persisting raw `resource.<ext>`.

## Data Contracts
Core orchestrator workflows:
1. Local read/write: get/save/list local resources.
2. Remote read/write: get/create/update/delete/list remote resources.
3. Orchestration workflows: apply, refresh, explain, diff, template, request.
4. Repository administration: init, refresh, push, reset, check, status.

Policy contract:
1. List workflows MUST accept explicit recursion policy and default to non-recursive behavior.

Resolution contract:
1. Input path -> metadata resolution -> `resource.Resource` identity resolution -> operation spec -> execution.
2. Optional secret masking/resolution performed at boundaries.
3. Direct request workflows MUST preserve the full rendered request contract, including metadata-derived query parameters, headers, `Accept`, and `Content-Type`, through managed-service execution.
4. Repository-backed resource resolution MUST treat resolved metadata `resource.defaults` content as part of the local desired payload for that same logical path rather than as a separate resource target.

## Failure Modes
1. Metadata resolved but required remote identity missing.
2. Remote fetch/mutation succeeds but local persist fails in workflows that write repository state.
3. Remote fallback candidates are ambiguous.
4. Repository sync conflict blocks local persistence workflows after successful remote operations.
5. Local metadata-id fallback yields multiple candidates for one path segment.
6. Metadata defaults content or defaults artifacts are invalid for the effective payload type or conflict with the canonical payload descriptor.

## Edge Cases
1. Direct get path fails and alias-based fallback succeeds.
2. Alias changes between local and remote while remote ID remains stable.
3. Remote list returns duplicate alias candidates.
4. Dry-run explain differs from live apply due to dynamic template context drift.
5. Local path segment is an ID while repository uses metadata alias for stored logical paths.
6. Collection endpoint returns `404` when empty and must be interpreted as zero items rather than missing endpoint when collection hints are present and the parent resource exists.
7. Nested collection read returns `404` because the parent resource is missing and must remain `NotFound` instead of collapsing to `[]`.
8. A local resource with `/customers/_/metadata.yaml` referencing `defaults.yaml` and `/customers/acme/resource.yaml` can diff cleanly against a remote payload even when `resource.yaml` omits most fields because compare uses the effective merged payload, not the override file alone.

## Examples
1. `Apply(/customers/acme)` resolves metadata, reads remote resource state, compares with metadata compare transforms, then creates/updates only when drift exists (or when force is enabled).
2. `Refresh(/customers)` lists remote collection, maps each item to deterministic alias paths, and writes local files.
3. `Diff(/customers/acme)` loads local and remote payloads, applies compare transforms, and returns deterministic operations.
4. `Apply(/admin/realms/master/clients/<uuid>)` resolves the local resource by metadata ID fallback when only alias-based repository paths exist.
5. `GetRemote(/admin/realms/master/organizations)` returns an empty list when the server responds `404` for an empty collection and collection hints confirm the path is a collection.
6. `GetRemote(/admin/realms/acme/organizations)` preserves `NotFound` when `/admin/realms/acme` is also missing, even if OpenAPI hints confirm `organizations` is a collection endpoint.
7. `Apply(/customers/acme)` with `/customers/_/metadata.yaml` referencing `defaults.yaml` and `/customers/acme/resource.yaml` resolves identity and validation from the merged payload, but later local saves still keep only the non-default overrides in `resource.yaml`.
