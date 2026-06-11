# Orchestrator and Integration

## Purpose
Coordinate repository, metadata, managed-service, and secret operations for all user workflows (apply/refresh/diff/explain/template/request and local/remote read/write).

## Normative Rules
1. `orchestrator.Orchestrator` MUST be the only component that coordinates multiple managers in one workflow.
2. Every workflow MUST declare mutation scope: local, remote, or both.
3. Identity-template, compare-directive, `resource.defaults`, required-attribute, and validate semantics are defined in `agents/reference/metadata.md`; remote request construction, auth, and list-`jq` resolution in `agents/reference/managed-service.md`. This file enforces them in the flows below.

### Resolution pipeline
4. Every workflow MUST follow: input path -> metadata resolution -> `resource.Resource` identity resolution -> operation spec -> execution, applying secret masking/resolution only at boundaries.
5. Apply-like operations MUST resolve metadata and identity before any remote mutation.
6. Local read, diff, template, and mutation-preparation MUST compute the effective local desired payload (after `resource.defaults` resolution and repository artifact expansion) before identity resolution, validation, secret resolution, compare transforms, or remote HTTP execution. Resolved `resource.defaults` content is part of the local desired payload for the same logical path, not a separate resource target.
7. Workflows that persist local state MUST use a normalized `resource.Resource` and payload.

### Apply / diff
8. Apply MUST read remote state first, create only on `NotFound`, and otherwise compare desired-vs-remote via metadata compare directives before deciding to update.
9. Apply MUST skip update when compare output shows no drift, unless forced.
10. Repeated apply with unchanged desired state MUST produce no additional mutations unless forced (idempotence).
11. Compare/diff output MUST be deterministic and stable for identical inputs.
12. Binary payload comparison MUST be whole-payload: identical bytes mean no drift; differing bytes MUST produce exactly one deterministic root-level replace diff entry.

### Local <-> remote fallback (deterministic and bounded; no unbounded search loops)
13. Single-resource local read MUST try literal repository lookup first; on `NotFound`, MUST fall back to bounded collection lookup by metadata `resource.id`, using reverse matching only when the identity template is a simple single-pointer expression.
14. Remote delete SHOULD attempt literal delete first and MAY retry once with metadata-aware identity fallback after `NotFound`.
15. Remote read SHOULD treat a `NotFound` collection read as an empty collection only when repository structure hints or OpenAPI inference indicate the path is a collection endpoint; it MUST preserve `NotFound` when a nested collection read fails because the parent resource is also `NotFound`.
16. Remote read metadata fallback MAY accept a single-candidate list result when metadata declares list `jq` filtering, but only when the requested logical path depth does not exceed the resolved selector/collection template depth. Singleton fallback MUST NOT collapse explicit child identity segments and SHOULD resolve to canonical remote identity for follow-up reads when possible.

### List / request / write
17. List workflows MUST accept an explicit recursion policy and default to non-recursive.
18. Direct request workflows MUST preserve the full rendered request contract (metadata-derived query parameters, headers, `Accept`, `Content-Type`) through managed-service execution.
19. Repository-backed write MUST preserve metadata-managed defaults layout by compacting the effective desired payload against the resolved metadata defaults object before persisting raw `resource.<ext>`.

### Errors
20. Conflict conditions MUST return typed `ConflictError` with actionable context.

## Failure Modes
1. Metadata resolved but required remote identity missing.
2. Remote mutation succeeds but local persist fails in a repository-writing workflow.
3. Remote fallback candidates ambiguous; local metadata-id fallback yields multiple candidates for one path segment.
4. Repository sync conflict blocks local persistence after a successful remote operation.
5. Metadata defaults content/artifacts invalid for the effective payload type or conflicting with the canonical payload descriptor.
6. Dry-run explain diverges from live apply due to dynamic template context drift.

## Examples
1. `Apply(/customers/acme)`: resolve metadata + identity, read remote, compare via compare transforms, then create/update only on drift (or when forced).
2. `Apply(/customers/acme)` with `/customers/_/metadata.yaml` referencing `defaults.yaml` and a sparse `/customers/acme/resource.yaml`: identity, validation, and diff use the merged effective payload; saves persist only the non-default overrides back into `resource.yaml`.
3. `Apply(/admin/realms/master/clients/<uuid>)`: resolves the local resource by metadata-`resource.id` fallback when only alias-based repository paths exist (alias may differ between local and remote while remote ID stays stable).
4. `Refresh(/customers)`: list remote collection, map each item to a deterministic alias path, write local files.
5. `Diff(/customers/acme)`: load local + remote payloads, apply compare transforms, return deterministic operations.
6. `GetRemote(/admin/realms/master/organizations)`: returns `[]` when the server answers `404` for an empty collection and collection hints confirm a collection endpoint.
7. `GetRemote(/admin/realms/acme/organizations)`: preserves `NotFound` when parent `/admin/realms/acme` is missing, even if OpenAPI hints mark `organizations` as a collection.
