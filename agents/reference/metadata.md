# Metadata Semantics, Layering, and Templates

## Purpose
Define deterministic metadata structure, layering/precedence, template rendering, inference, defaults, identity, format, secret declaration, externalized attributes, and descendant selectors. This file OWNS these concepts; other files reference it.

## Normative Rules

### Resolution and precedence
1. Metadata resolution MUST be deterministic and reproducible.
2. Source precedence MUST be: engine defaults, then configured shared source (`metadata.baseDir|bundle|bundleFile`), then repo-local overlays rooted at the active repository baseDir. Repo-local overlays MUST override shared-source directives for the same logical path.
3. Inside one source, precedence MUST be: inherited collection layers, wildcard layers, literal layers, resource layer. Resource-level directives MUST override inherited directives in the same source.
4. At the same depth, wildcard candidates MUST be applied before literal candidates. At the same precedence level, merge order MUST be stable and lexicographic.
5. Template rendering failures MUST return typed validation errors with path context.
6. Compare transforms MUST run before diff generation.

### Persistence and schema
7. Persistence MUST omit nil directive fields (never write `null`) and MUST preserve explicit empty arrays/maps (replacement semantics).
8. Persistence MUST preserve raw `resource.defaults` include placeholders on read/write and MUST NOT inline included defaults objects into stored `metadata.yaml|json`.
9. Repo-local resource metadata MUST be `metadata.yaml|json` sidecars next to `resource.<ext>`; repo-local collection metadata MUST be `/<collection>/_/metadata.yaml|json`. When both extensions exist, `metadata.yaml` MUST win on read. Persistence SHOULD write `metadata.yaml` by default and remove a superseded sibling sidecar after success.
10. Mutation workflows (`get/set/unset/edit` plus defaults-artifact writes) MUST target: the explicit `metadata.baseDir` source when configured (even if a repository baseDir also exists); otherwise, when the active shared source is `metadata.bundle|bundleFile` and a repository baseDir exists, the repo-local overlay (leaving the bundle cache read-only); otherwise the repository-local tree.
11. Changes to the persisted metadata wire shape or validation contract MUST update `schemas/metadata.schema.json` in the same change; that schema MUST describe the canonical nested structure and MUST NOT reintroduce legacy flat aliases.

### Defaults (`resource.defaults`)
12. Resolution MUST expand include-backed `resource.defaults.value` and `resource.defaults.profiles[*]` from selector-local deterministic files named `defaults.<ext>` or `defaults-<profile>.<ext>`, MUST support only `json|yaml|yml|properties`, and MUST require structured object payloads.
13. `value` and each `profiles[*]` entry MUST accept either one inline structured object or one exact include placeholder; profile names MUST be filename-safe and deterministic because include filenames derive from the profile key.
14. `mode` MUST support `inherit`, `ignore`, `replace`. `ignore` drops inherited baseline defaults and inherited selected profiles while keeping ancestor profile catalogs available for explicit local `useProfiles`. `replace` additionally drops ancestor profile catalogs.
15. `useProfiles` MUST inherit when omitted, MUST clear when explicitly empty (`[]`), and selected profiles MUST apply in listed order before local `resource.defaults.value`.

### Identity (`resource.id`, `resource.alias`)
16. `resource.id` and `resource.alias` MUST be identity template strings, MUST NOT accept legacy identity fields, MUST accept raw JSON Pointer shorthand `/id` and one-level shorthand `{{id}}` as equivalent to `{{/id}}`, and MAY embed one or more RFC 6901 JSON Pointer expressions plus helpers `uppercase`, `lowercase`, `trim`, `substring`, `default`.
17. When `resource.id` or `resource.alias` is omitted, identity resolution MUST default that field to `/id`.
18. Identity rendering MUST fail with a typed validation error when a referenced pointer is missing unless a helper such as `default` handles it. Complex multi-pointer templates MUST support forward rendering but MUST NOT be reverse-mapped; reverse mapping is limited to single-pointer templates such as `{{/id}}`.

### Format (`resource.format`)
19. `resource.format` MAY define one concrete payload format or `any`. Concrete values MUST support `json`, `yaml`, `xml`, `hcl`, `ini`, `properties`, `text`, `octet-stream`; concrete values MAY drive default repository save suffix and request media defaults when no explicit descriptor/header wins. `format: any` MUST preserve mixed repository/request descriptors instead of coercing one collection to one format.
20. When concrete `resource.format` resolves to a non-structured payload type, `resource.defaults`, `resource.id`, `resource.alias`, `resource.requiredAttributes`, `resource.secretAttributes`, and `resource.externalizedAttributes` MUST fail validation (they require structured traversal). `format: any` MUST defer that validation until a concrete descriptor is known.

### Required attributes (declaration side)
21. `resource.requiredAttributes` MUST be preserved through merge/render/serialization and MUST remain resource-scoped.
22. Body-bearing mutation workflows MUST validate the structured source payload against the effective required-attribute set before operation-specific payload transforms run. The effective set MUST include every pointer referenced by explicitly configured `resource.alias`; non-create mutations MUST also include every pointer referenced by explicitly configured `resource.id`, even when `resource.requiredAttributes` omits them. Create MAY omit metadata-derived `resource.id` pointers unless explicitly declared by `resource.requiredAttributes` or operation validation.

### Secret declaration side
23. `resource.secret: true` MAY declare one whole-resource secret save/read contract for that scope, MUST remain distinct from attribute-scoped secret masking, and MUST be mutually exclusive with `resource.secretAttributes`. Secret lifecycle/masking semantics are owned by secrets.md.

### Externalized attributes (`resource.externalizedAttributes`)
24. Entries MUST default unspecified `template|mode|saveBehavior|renderBehavior|enabled` deterministically and MUST validate duplicate enabled `path` or `file` entries before persistence or workflow use.
25. Enabled entries MUST treat `path` as one JSON Pointer traversal path, traversing object keys by token and arrays only through zero-based numeric tokens or `*` wildcard tokens; MUST reject empty paths/files and repository-escaping relative files; MUST externalize only text/string payload values (MVP scope); MUST leave disabled entries inert.
26. When an enabled `path` uses one or more `*` wildcard array tokens, repository workflows MUST materialize concrete artifact file names deterministically by appending matched wildcard indices before the configured extension (for example `script.sh` -> `script-0.sh`), and placeholder rendering/expansion MUST use that concrete file path.
27. Repository-backed payload workflows (`save`, `apply`, `create`, `update`, `diff`) MUST replace configured include placeholders with sidecar file contents before downstream payload transforms or identity resolution, only when the stored value matches the configured placeholder template exactly. A configured-but-missing sidecar file MUST fail with a typed validation error before remote HTTP execution.

### Operations, paths, and templates
28. `resource.remoteCollectionPath` templates MUST support indirection by resolving template fields from the handled logical path when payload attributes are absent. When omitted, the effective remote collection path MUST default to the handled logical collection path.
29. Operation paths starting with `.` (for example `.` or `./{{/id}}`) MUST resolve relative to the rendered effective collection path. When an operation path is omitted, defaults MUST be `.` for `create` and `list`, and `./{{/id}}` for `get`, `update`, `delete`, `compare`.
30. Template-rendered string fields MUST support canonical JSON Pointer placeholders `{{/id}}`, MUST accept one-level shorthand `{{id}}` as equivalent, MAY accept legacy dot notation `{{.id}}` for compatibility, and MUST support `{{payload_type .}}`, `{{payload_media_type .}}`, `{{payload_extension .}}`, which resolve from the active payload descriptor without forcing a media default when no concrete descriptor is available.
31. Template scopes MUST expose `logicalCollectionPath` (handled logical collection path) and `remoteCollectionPath` (effective remote collection path), and MUST expose `contentType` as the active payload media type when the payload does not already define `contentType` (so media-aware templates work for raw text/octet-stream payloads).
32. Metadata defaults MUST leave media header selection to payload-type-aware request building unless explicit metadata overrides are present.
33. List-operation `transforms[*].jqExpression` MAY call `resource("<logical-path>")`; resolution MUST target the same active source as the primary list workflow and return normalized JSON payload.

### Operation validation directives
34. `validate.requiredAttributes`, `validate.assertions`, `validate.schemaRef` MUST be preserved through merge/render/serialization and MUST remain operation-scoped.

### Inference
35. Inference SHOULD propose method/path defaults and SHOULD infer `resource.format` from managed-service payload formats (using `any` when more than one deterministic format is advertised), but MUST NOT overwrite explicit user metadata unless explicitly requested.
36. Inference MUST accept selector paths with intermediary `_` segments and trailing collection markers (for example `/admin/realms/_/clients/`).
37. Selector-path inference SHOULD use OpenAPI path templates to infer operation paths and identity templates; non-template-safe OpenAPI parameter names MUST fall back to deterministic placeholder names from fallback inference.
38. Inference output SHOULD omit directives equal to deterministic fallback defaults.
39. OpenAPI-backed inference SHOULD populate `operations.create/update.validate.schemaRef` as `openapi:request-body` when request-body schemas exist and MAY populate `validate.requiredAttributes` from deterministic schema `required` fields.
40. Until recursive traversal is implemented, inference requests with `recursive=true` MUST return a typed validation error and MUST NOT persist metadata changes.

### Descendant selectors (`selector.descendants`)
41. `selector.descendants` MAY be persisted only on collection metadata, MUST default to disabled, and MUST opt a non-root matched collection selector into deeper descendant inheritance beyond its exact collection root and immediate resource items.
42. Non-root collection metadata without `selector.descendants: true` MUST apply only to the matched collection root and its immediate resource items; the root selector `/` remains global.
43. When multiple descendant-enabled collection selectors apply, render-time descendant scope MUST use the deepest matched collection root after normal source and specificity precedence.
44. Descendant-enabled template scope MUST expose `descendantPath` and `descendantCollectionPath` as slash-prefixed suffixes from the matched collection root to the handled target path or target collection path, and MUST expose `""` when the handled target is exactly at that root.
45. Descendant scope fields MUST remain render-only helpers and MUST NOT be merged into payload mutation input, required-attribute validation input, or effective resolved metadata snapshots returned by `ResolveForPath`.

## Data Contracts
Metadata groups (beyond interfaces.md):
1. `selector`: persisted collection-selector directives (`descendants`) that gate deep inheritance but do not merge into resolved metadata.
2. `resource`: identity, required-attribute, format, defaults, whole-resource-secret, secret-attribute, and collection directives.
3. `resource.defaults`: `mode`, `useProfiles`, `value`, `profiles`; `value` and `profiles[*]` accept one inline object or one deterministic include placeholder.
4. `resource.externalizedAttributes[*]`: `path` (one JSON Pointer; arrays use numeric or `*` tokens), `file`, optional `template|mode|saveBehavior|renderBehavior|enabled`.
5. `operations.create/update/delete/get/compare/list`: operation-specific directives. `operations.defaults.transforms`: shared ordered pipeline applied before operation-specific pipelines.
6. Operation wire fields: `path`, `method`, `query`, `headers`, `body` (media headers `Accept`/`Content-Type` are `headers` entries).
7. Transform wire fields: `selectAttributes`, `excludeAttributes`, `jqExpression`.
8. Operation validation wire fields: `validate.requiredAttributes`, `validate.assertions[*].{message,jq}`, `validate.schemaRef`.
9. Resource-level fields: `requiredAttributes`, `secret`, `secretAttributes`.

Operation selector: API boundaries MUST use typed `metadata.Operation`; allowed values are `get`, `create`, `update`, `delete`, `list`, `compare`.

Infer options: `apply` (whether inferred directives persist); `recursive` (reserved — current behavior MUST reject `true`).

Template context: current and ancestor payload fields; context attributes (logical path, logical collection path, effective remote collection path, alias, remote ID); `../` ancestor traversal; placeholder/helper forms per rules 30-31; descendant helpers per rule 44; `contentType` alias per rule 31.

## Layering Algorithm
1. Start with engine defaults.
2. Collect matching collection metadata candidates from root to target collection.
3. Apply root selector `/` globally.
4. For each depth, apply wildcard then literal candidate in stable order.
5. Non-root collection selectors apply only at their matched root and immediate resource items unless `selector.descendants` is enabled.
6. Apply target resource metadata last.
7. Normalize merged result and validate required fields for the requested operation.

## Failure Modes
1. Missing required operation path after all layers resolve.
2. Invalid/unresolvable template variables or relative references exceeding ancestor depth.
3. Transform expressions producing invalid payload shapes.
4. Conflicting metadata causing ambiguous identity resolution.
5. Externalized-attribute `file` paths containing `../`, or duplicate enabled `file`/`path` entries, fail validation deterministically before repository IO.
6. Identity templates referencing a missing pointer without a `default` helper (rule 18).

## Edge Cases
1. `secretAttributes` pointing to missing payload fields SHOULD NOT fail metadata resolution.
2. Explicit null directive clears an inherited field; compare suppression removing all fields yields an empty normalized payload.
3. Selector-path inference without OpenAPI data still returns deterministic fallback hints, anchored to the matched collection root so logical suffix folders under a descendant-enabled selector do not create bogus fields (for example `secret=path`).
4. `resource("<logical-path>")` list-jq lookups resolve parent resources through metadata-aware alias/id fallback, then filter candidates deterministically by referenced fields.
5. Wildcard array externalization skips elements lacking the targeted attribute while still materializing indexed sidecars for matching siblings only.
6. Repository payloads MAY keep inline values for configured externalized attributes; expansion occurs only when the stored value matches the configured placeholder string exactly.
7. `format: octet-stream` disables structured payload transforms and validation for that scope; `text`/`octet-stream` MAY use `resource.secret: true` but reject `resource.secretAttributes`, identity templates, and externalized attributes.
8. Invalid helper usage (for example `{{payload_type "yaml"}}`) returns a typed validation error.

## Examples
1. `/customers/_` defines `operations.get.path: /api/customers/{{/id}}`; `/customers/acme/metadata` overrides only `operations.get.headers`. Compare suppression: `operations.compare.transforms: [{excludeAttributes:["/updatedAt","/version"]}]`.
2. Inference for `/admin/realms/_/clients/` proposes `resource.id: {{/id}}`, `resource.alias: {{/clientId}}`, and templated operation paths from OpenAPI selectors. `--recursive` MUST fail with a validation error and write nothing.
3. Selector `/admin/realms/_/user-registry` with `resource.remoteCollectionPath: /admin/realms/{{/realm}}/components` and `operations.get.path: ./{{/id}}`: rendering `/admin/realms/platform/user-registry` with `id=123456` resolves to `/admin/realms/platform/components/123456`.
4. Selector `/projects/_/jobs/_`: omitting `resource.remoteCollectionPath` defaults remote access to `/projects/{{/project}}/jobs`; if the remote is `/project/{{/project}}/jobs`, metadata MUST set `resource.remoteCollectionPath` to that value while `project` still resolves from the logical collection path.
5. Selector `/admin/realms/_/user-registry/_/mappers/`: `operations.list.transforms: [{jqExpression:"..."}]` MAY call `resource("/admin/realms/{{/realm}}/user-registry/{{/provider}}/")` and compare mapper `parentId` with resolved parent `.id`.
6. Selector `/projects/_/secrets/_` with `selector.descendants: true`, `resource.remoteCollectionPath: /storage/keys/project/{{/project}}{{/descendantCollectionPath}}`, `operations.get.path: ./{{/id}}`: rendering `/projects/platform/secrets/path/to/db-password` resolves to `/storage/keys/project/platform/path/to/db-password`; `operations.list.path: .` on `/projects/platform/secrets/path/to` resolves to `/storage/keys/project/platform/path/to`.
7. `/customers/_/metadata.yaml` declares `resource.defaults.value: "{{include defaults.yaml}}"` plus `resource.defaults.profiles.prod: "{{include defaults-prod.yaml}}"`; resolution expands those files into structured objects before merge with `resource.yaml`. Child `useProfiles: []` clears inherited profile selection while keeping the inherited baseline. Child `mode: ignore` MAY select one ancestor profile explicitly; `mode: replace` must redefine any needed profile locally.
8. `resource.externalizedAttributes: [{path:"/script", file:"script.sh"}]` plus `script: "{{include script.sh}}"` stores content in sibling `script.sh` and expands it back for apply/diff; a missing `script.sh` fails with a typed validation error before remote HTTP. Wildcard `path:"/sequence/commands/*/script"` stores `{{include script-0.sh}}`, `{{include script-2.sh}}` for matching elements only.
9. `resource.format: text` plus `resource.secret: true` is valid (whole-file secret); `resource.secretAttributes: ["/password"]` at that scope fails validation. `format: any` on `/customers/_` lets `/customers/acme/resource.yaml` and `/customers/beta/resource.json` coexist without coercion.
10. `resource.id: "{{/id}}"`, `resource.alias: "{{/clientId}}"`, `resource.requiredAttributes: ["/realm"]`: create with `/realm` and `/clientId` succeeds when the server assigns `/id`, but update still requires `/id` unless operation validation provides another source. The alias pointer `/clientId` stays required even when `operations.update.transforms` later excludes it from the transmitted body.
11. `operations.get.path: /api/customers/{{id}}` is valid shorthand for `{{/id}}`; docs and inferred output SHOULD prefer canonical `{{/id}}`. `resource metadata get` preserves helper tokens (for example `{{payload_media_type .}}`); `resource get --show-metadata` renders them against the active payload descriptor.
12. Raw `resource.key` payload renders `{{index . "contentType"}}` because the scope injects `contentType: application/octet-stream` from the active descriptor. OpenAPI inference for `application/octet-stream` media infers `resource.format: octet-stream` when explicit metadata is absent. `operations.create.validate.requiredAttributes: ["/realm"]` is satisfied for `/admin/realms/platform/...` when `realm` derives from the logical path template context.
