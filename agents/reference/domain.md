# Domain Vocabulary and Invariants

## Purpose
Define shared business vocabulary and the canonical, non-negotiable invariants other files enforce.

## Vocabulary
1. Logical Path: canonical normalized-absolute repository path for a resource.
2. Collection Path: parent path segment representing a collection of resources.
3. Alias: human-friendly stable key used for local path selection within a collection.
4. Remote ID: server-facing identifier used in operation paths.
5. Template Context: data scope used to render metadata templates.
6. `_`: reserved segment naming the metadata namespace; never a resource identifier.
7. Source of truth: local repository state is desired state (apply); remote server state is observed state (refresh).

Type/interface contracts for these entities are defined in agents/reference/interfaces.md.

## Normative Rules

### Identity and aliases
1. Logical resource paths MUST be normalized absolute paths; segments violating normalization MUST be rejected.
2. Resource identity MUST be derived by explicit metadata rules before any fallback heuristic.
3. Alias resolution MUST be deterministic within a collection scope; a non-unique alias in the same collection is a conflict and MUST be surfaced.
4. `resource.id` and `resource.alias` are logical-segment identities; nested descendant branches MUST use template scope helpers (e.g. `descendantPath`) instead of slashful rendered IDs. Template rendering and helpers are defined in agents/reference/metadata.md.
5. The `_` segment MUST be treated as metadata namespace and never as a resource identifier.

### Metadata precedence (canonical)
6. Root collection metadata is global. Non-root collection metadata applies only to its matched collection root and immediate resource items, unless `selector.descendants: true`, which extends it to deeper descendants. Resource-level metadata overrides collection-level metadata.
7. Repo-local metadata overlays override shared metadata-source directives for the same logical path, regardless of selector specificity across sources.
8. Array fields in metadata are replace, not deep-merge. The layering algorithm is defined in agents/reference/metadata.md; this file fixes the precedence and merge semantics it must satisfy.

### Required-attribute invariant (canonical)
9. Structured body-bearing resource mutations MUST require metadata `resource.requiredAttributes`.
10. Every JSON Pointer referenced by configured `resource.alias` MUST count as required even when `resource.requiredAttributes` omits it.
11. Every JSON Pointer referenced by configured `resource.id` MUST count as required for non-create mutations; create MAY omit metadata-derived `resource.id` pointers unless explicitly declared elsewhere (e.g. server-assigned IDs).
12. A required attribute MUST be present in the source payload even if an operation transform later excludes it from the transmitted body.

### Defaults-merge invariant (canonical)
13. Local desired state for one logical resource MAY be composed from resolved metadata `resource.defaults` plus raw `resource.<ext>` overrides; object fields deep-merge, arrays replace, and explicit override values win deterministically.
14. `resource.defaults` baseline values and profiles MUST represent stable desired-state defaults safe to resend to the managed service and MUST NOT carry volatile observed-only fields (timestamps, versions, generated IDs, status blocks).

### format:any invariant (canonical)
15. Collection metadata `resource.format: any` MUST let child resources in the same collection preserve or supply different payload descriptors during save, instead of coercing the whole collection to one repository format.

### Operation consistency
16. Metadata directives MUST be applied consistently across get/create/update/delete/list/compare.
17. Compare MUST ignore fields declared by metadata suppression/filter rules.

Secret declaration (`resource.secret`, `secretAttributes`) is defined in agents/reference/metadata.md; secret lifecycle in agents/reference/secrets.md. This file treats a payload mixing secret and non-secret siblings as valid input both layers must handle.

## Failure Modes
1. Alias collision -> ambiguous target resolution, MUST be surfaced.
2. Missing metadata required to build remote paths.
3. Divergent local vs remote identity -> unintended updates (e.g. remote resource exists but local alias changed).
4. Unsupported path segments violating normalization.
5. Empty collection while metadata inference is still required.

## Examples
1. Required-attribute survives transform: metadata `resource.alias: "{{/clientId}}"`, `resource.requiredAttributes: [/realm]` still requires `/clientId` in a structured create/update payload even when a transform later excludes `/clientId` from the transmitted body.
2. Server-assigned ID: metadata `resource.id: "{{/id}}"`, `resource.alias: "{{/clientId}}"`, `resource.requiredAttributes: [/realm]` can create from `{realm, clientId}` when the server assigns the ID, but update still requires `/id` unless another layer supplies it.
3. format:any: `resource.format: any` on `/customers/_` lets one collection save keep `/customers/acme/resource.yaml` and `/customers/beta/resource.json` instead of normalizing both to one suffix.
4. Defaults merge: `/customers/_/metadata.yaml` declares `resource.defaults.value: "{{include defaults.yaml}}"` with `/spec/enabled: true`; `/customers/acme/resource.yaml` keeps only the override `/spec/enabled: false`. Runtime desired state is the merged object and the explicit `false` MUST win and stay persisted in `resource.yaml`.
