# Domain Vocabulary and Invariants

## Purpose
Define shared business language and non-negotiable invariants so behavior remains consistent across modules.

## In Scope
1. Domain terms and meanings.
2. Business invariants and conflict rules.
3. Identity and alias semantics.
4. Source-of-truth model.

## Out of Scope
1. CLI presentation details.
2. Adapter-specific protocol knobs.
3. Build and deployment concerns.

## Normative Rules
1. Local repository state is the source of desired state for apply workflows.
2. Remote server state is the source of observed state for refresh workflows.
3. Logical resource paths MUST be normalized absolute paths.
4. Resource identity MUST be derived by explicit metadata rules before fallback heuristics.
5. Alias resolution MUST be deterministic within a collection scope.
6. Metadata directives MUST be applied consistently for get/create/update/delete/list/compare operations.
7. The reserved segment `_` MUST be treated as metadata namespace and not a resource identifier.

## Data Contracts
Domain entities:
1. `Resource`: desired or observed payload.
2. `ResourceMetadata`: operation and transform directives.
3. `resource.Resource`: identity, paths, metadata, and payload bundle.
4. `orchestrator.DefaultOrchestrator`: active runtime managers and configuration identity.

Key terms:
1. Logical Path: canonical repository path for a resource.
2. Collection Path: parent path segment representing a collection.
3. Alias: human-friendly stable key used for local path selection.
4. Remote ID: server-facing identifier used in operation paths.
5. Template Context: data scope used to render metadata templates.

## Business Rules
1. Collection metadata applies by inheritance only where explicitly allowed.
2. Resource-level metadata overrides collection-level metadata.
3. Repo-local metadata overlays override shared metadata-source directives for the same logical path, regardless of selector specificity across sources.
4. Array fields in metadata are replace, not deep-merge.
5. Compare behavior MUST ignore fields declared by metadata suppression/filter rules.
6. Non-unique alias in the same collection is a conflict and MUST be surfaced.
7. Structured body-bearing resource mutations MUST require metadata `resource.requiredAttributes`; every JSON Pointer referenced by configured `resource.alias` MUST count as required even when `resource.requiredAttributes` omits it; and every JSON Pointer referenced by configured `resource.id` MUST count as required for non-create mutations while create MAY omit metadata-derived `resource.id` pointers unless they are explicitly declared elsewhere.
8. Local desired state for one logical resource MAY be composed from resolved metadata `resource.defaults` plus raw `resource.<ext>` overrides; object fields deep-merge, arrays replace, and explicit override values win deterministically.
9. Collection metadata `resource.format: any` MUST allow child resources in the same collection to preserve or supply different payload descriptors during save workflows instead of coercing the whole collection to one repository format.
10. `resource.defaults` baseline values and profiles MUST represent stable desired-state defaults that are safe to resend to the managed server and MUST NOT be used for volatile observed-only fields such as timestamps, versions, generated IDs, or status blocks.

## Failure Modes
1. Alias collision causing ambiguous target resolution.
2. Missing metadata fields required to build remote paths.
3. Divergent local and remote identity causing unintended updates.
4. Unsupported path segments violating normalization rules.

## Edge Cases
1. Resource exists remotely but local alias changed.
2. Metadata wildcard applies to nested descendants with partial overrides.
3. Resource payload contains fields with both secret and non-secret siblings.
4. Collection has zero items and metadata inference still required.
5. Structured update transforms remove an alias field from the outgoing body after resource-level validation already confirmed it exists in the source payload.
6. Structured create payloads can omit a server-assigned `resource.id` while update still requires that ID when metadata declares it.
7. `resource.format: any` on `/customers/_` lets `/customers/acme` keep `resource.yaml` while `/customers/beta` keeps `resource.json` during one collection save.
8. A resource stores `/spec/enabled: false` in `/customers/acme/resource.yaml` while `/customers/_/metadata.yaml` references `{{include defaults.yaml}}` with `/spec/enabled: true`; the explicit override MUST win and stay persisted in `resource.yaml`.

## Examples
1. Local path `/customers/acme` maps to collection `/customers`, alias `acme`, and remote ID from rendered `resource.id` when configured.
2. Metadata on `/customers/_` sets default `operations.get.path`; resource metadata on `/customers/acme` overrides only `operations.update.path`.
3. Diff operation suppresses `/updatedAt` and `/lastSeen` before comparison to avoid false drift.
4. Metadata with `resource.alias: "{{/clientId}}"` and `resource.requiredAttributes: [/realm]` still requires `/clientId` in a structured create/update payload even when an operation transform later excludes `/clientId` from the transmitted body.
5. Metadata with `resource.id: "{{/id}}"`, `resource.alias: "{{/clientId}}"`, and `resource.requiredAttributes: [/realm]` can create a resource from `{realm, clientId}` when the server assigns the ID, but update still requires `/id` unless another validation layer supplies it.
6. `resource.format: any` on `/customers/_` allows one collection save to preserve `/customers/acme/resource.yaml` and `/customers/beta/resource.json` instead of normalizing both to one suffix.
7. `/customers/_/metadata.yaml` can declare `resource.defaults.value: "{{include defaults.yaml}}"` for shared fields such as labels and policy defaults while `/customers/acme/resource.yaml` keeps only the differing overrides; runtime desired state is the merged object, not either file alone.
