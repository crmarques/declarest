# Testing, Quality, and Security

## Purpose
Define quality gates and security invariants so behavior changes are verifiable and safe.

## In Scope
1. Test strategy by risk level.
2. Security and safety controls.
3. Required regression/acceptance coverage.
4. Release-readiness checks.

## Out of Scope
1. CI vendor configuration.
2. Runtime observability platform setup.
3. UI style concerns.

## Normative Rules
1. Every behavior change MUST add tests at the lowest effective layer.
2. High-risk orchestration or integration changes MUST include integration coverage.
3. CLI contract changes MUST include command-level success and failure tests.
4. Security-sensitive flows MUST include negative tests.
5. Deterministic ordering guarantees MUST be asserted.
6. Path traversal protections MUST be tested for repository and metadata access.
7. Secret values MUST never appear in logs, errors, or snapshots.
8. New normative rules SHOULD include an explicit matching test expectation.

## Data Contracts
Test layers:
1. Unit: pure transforms, normalization, metadata layering/template rendering, secret placeholder normalization.
2. Integration: orchestrator workflows with fake providers and conflict handling.
3. E2E: CLI workflows using representative stacks and fixture trees.

Acceptance contracts:
1. Orchestrator idempotency for repeated apply.
2. Stable diff ordering for equivalent inputs.
3. Typed error categories for all major failure classes.

## Required Scenario Coverage
1. Metadata precedence: wildcard/literal collisions and template resolution (including relative references).
2. Repository safety: traversal rejection and deterministic list/delete recursion behavior.
3. Identity handling: alias/ID divergence and ambiguity conflict detection.
4. Secrets lifecycle: detect/mask/resolve/normalize behavior and non-disclosure guarantees.
5. Compare/diff semantics: suppression/filter rules, stable ordering, structured diff/explain `resource.DiffEntry` path shape (`ResourcePath` + RFC 6901 `Path` pointer with `Path=""` for root replace), and stable text-output rendering for `resource diff --output text`.
6. CLI execution footer: resource mutation commands (`resource save|apply|create|update|delete`) and state-changing HTTP request commands (`resource request post|put|patch|delete|connect`) emit deterministic `[OK|ERROR] ...` status lines to stderr unless `--no-status` is set, interactive terminals apply bold color tags, `--no-color`/`NO_COLOR` disable ANSI color tags, nil payload outputs stay empty, state-changing commands suppress payload output by default, and `--verbose` restores that payload output.
7. CLI safeguards: validation errors, conflicting path inputs, and destructive-operation protections.
8. Context config: strict decode, one-of validation, overrides precedence, and missing-catalog behavior.
9. Metadata bundles: manifest validation (`bundle.yaml` required fields), shorthand name/version contract checks, secure tar.gz extraction safeguards, and deterministic cache reuse behavior.
10. Bundle OpenAPI wiring: `resource-server.http.openapi` precedence over bundle hints, fallback to `bundle.yaml declarest.openapi`, bundle-root/metadata-root OpenAPI files (`openapi.yaml|openapi.yml|openapi.json`), recursive bundle OpenAPI file discovery, and cross-origin OpenAPI fetches without leaked auth headers.
10. Remote operation construction: OpenAPI-assisted defaults with explicit metadata override precedence.
10. Repository sync: conflict classes, actionable outcomes, `repo status` output contract, and verbose worktree-detail coverage for `repo status --verbose`.
11. E2E profiles: `basic|full|manual` workload behavior, requirement filtering, and deterministic step statuses.
12. E2E runtime UX: grouped step reporting (`RUNNING|OK|FAIL|SKIP`) and actionable failure log pointers.
13. Resource-server fixtures: metadata `resourceInfo` identity mapping (`idFromAttribute`/`aliasFromAttribute`) and intermediary `/_/` expansion for nested trees.
14. E2E component orchestration: dependency-aware hook ordering, parallel ready-batch execution, and cycle/missing-dependency failures.
15. OAuth2 component auth: `client_credentials` token issuance and bearer-token rejection when auth is missing or invalid.
16. mTLS component auth: only configured client certificates are accepted when mTLS is enabled.
17. Basic-auth component auth: requests fail without valid credentials when basic auth is selected and succeed with configured username/password.
18. HTTP request CLI routing: `resource request <method>` maps to resource-server requests with positional/flag path validation and payload decoding from `--payload <path|->` or stdin, and `resource request post|put --payload` inline decoding with source-conflict validation.
19. mTLS trust reload: updating `simple-api-server` trusted client-cert files at runtime changes access behavior for new connections without service restart, including empty trusted-cert sets denying all access.
20. Resource save secret safeguard: `resource save` fails on potential plaintext secrets that are not metadata-declared unless `--ignore` is provided, metadata-declared candidates are automatically stored/masked before persistence (and fail when no secret provider is configured), and `resource save --handle-secrets[=<comma-separated-attributes>]` handles selected candidates, stores deterministic path-scoped secret keys, writes `{{secret .}}` placeholders, updates metadata, skips group items that do not contain requested attributes, and fails with warning when non-metadata-declared candidates remain unhandled.
21. Secret detect metadata fix flow: `secret detect` scans repository scope when no payload input is provided (default scope `/`), `secret detect --fix` merges detected attributes into metadata `resourceInfo.secretInAttributes`, and `--secret-attribute` filtering has negative validation coverage for payload and repository-scan modes.
22. Resource collection-target scope: `resource apply|create|update|delete|diff` execute direct-child collection targets by default; `apply|create|update|delete` include descendants with `--recursive`; `resource diff` flattens per-resource compare output into one deterministic list, and `resource apply|create|update` accept explicit payload input for single-target mutations while repository-driven mode loads payloads for resolved targets when explicit input is absent; explicit-payload mutations MUST include CLI coverage for collection-path child inference from metadata alias/id plus compatibility coverage for the equivalent explicit resource path.
23. Resource metadata HTTP-method override: `resource get|list|apply|create|update|delete --http-method <METHOD>` overrides metadata operation `method` for remote operations, rejects repository-only source selections where no remote request is executed, and keeps `resource apply` existence checks on default GET semantics while overriding only create/update writes.
24. Resource save list identity fallback: when list entries omit metadata-defined alias/id attributes, `resource save` falls back to common identity attributes (`clientId`, `id`, `name`, `alias`) before returning alias-resolution errors.
25. Metadata-aware read fallback: `resource get` and `resource save` (no input) attempt literal remote read first and then collection list/filter fallback by metadata alias/id when `NotFound`; `resource request get` mirrors this behavior after a `NotFound` literal request.
26. Resource copy source fallback: `resource copy` reads the source from the local repository first and retries from the remote server only when the local read returns `NotFoundError`, then preserves the resolved payload for save/override validation.
26. Metadata-aware path fallback breadth: repository-backed single-resource workflows (`resource get --source repository`, `resource apply`, `resource update`, `resource diff`, `resource explain`) use literal local lookup then bounded metadata `idFromAttribute` fallback; remote delete retries with metadata-aware identity after literal `NotFound`.
26. Resource secret placeholder resolution: remote workflows resolve `{{secret .}}` to logical-path scoped keys, resolve `{{secret <custom-key>}}` overrides, and remain compatible with legacy absolute key placeholders.
27. Metadata persistence normalization: metadata writes omit nil fields (no `null` serialization) while preserving explicit empty arrays/maps used for merge replacement behavior.
28. Resource save override guard: saving to an already-present logical path must fail without `--overwrite` and succeed when `--overwrite` is provided so repository state cannot be overwritten accidentally.
29. Resource save wildcard expansion: `resource save` must expand `_` path segments via remote direct-child list traversal, persist all matched collection/resource targets, reject wildcard+payload input, and return `NotFound` when no concrete matches are resolved.
30. CLI path completion: completion merges repository paths, remote paths, and OpenAPI paths with command-aware source priority (remote-first defaults for `resource get|save|list|delete` and `resource request <method>`, repository-first defaults for `resource apply|create|update|diff|explain|template`, `metadata *`, and path-aware `secret` commands; each uses fallback to the secondary source when the preferred source has no candidates); templated OpenAPI segments resolve concrete values through source-prioritized collection listings, placeholder segments (`{...}`) are excluded from rendered suggestions, alias-aware segments prefer metadata `aliasFromAttribute` values over IDs when available, rendered suggestions use canonical absolute paths that remain prefix-compatible with shell token filtering, collection-prefix suggestions preserve deterministic trailing-`/` semantics, scoped completion queries prefer parent-path direct listing with bounded recursive/root fallback only when needed, metadata selector-only child branches (for example intermediary `/_/` templates not represented in OpenAPI) are surfaced as canonical logical child suggestions when selectors match the typed path, and path completions emit `NoSpace` so accepted completions do not auto-append a trailing space.
31. CLI startup bootstrap gating: `version` and context-catalog commands (`config print-template|add|edit|update|delete|rename|list|use|show|current|resolve|validate`) execute without active-context resolution, while runtime commands (`resource/*` including `resource request <method>`, `resource-server/*`, `repo/*`, `metadata/*`, `secret/*`, and `config check`) still fail fast when no active context is available.
32. Config add input contract: `config add` defaults `--format` to `yaml`, accepts context name from positional arg or global `--context`, skips interactive name prompt when provided, and returns `ValidationError` when both names differ.
33. Config add interactive schema coverage: wizard prompts require resource-server capture, allow optional-section skipping for optional blocks, support full context attribute capture across repository/resource-server/secret-store/preference blocks, and enforce one-of prompt branching so only the selected auth/key-source/provider branch is collected.
34. Config template output contract: `config print-template` emits a stable commented YAML template containing all supported context options, explicitly documents mutually-exclusive sections, accepts no positional args, and runs without active-context resolution.
35. Repo command repository-type awareness: `repo push` and `repo commit` fail fast with `ValidationError` on filesystem contexts, `repo status` default text output differs by repository type while preserving stable structured (`json|yaml`) sync fields, `repo status --verbose` emits deterministic git-worktree details for git contexts, and `repo tree` returns a deterministic local directory-only tree view.
36. Repo commit CLI contract: `repo commit --message|-m` validates non-empty messages, creates one local git commit when the worktree is dirty, and returns deterministic success output when no changes are available to commit.
35. Repo clean contract: `repo clean` invokes repository cleanup, removes tracked and untracked uncommitted changes in git repositories, and succeeds as a no-op for filesystem repositories.
36. Context validation contract: all context-catalog mutation and resolve flows fail with `ValidationError` when `resource-server` is missing, interactive `config add` always prompts resource-server configuration, and `config edit` rejects invalid edited catalog/context YAML without persisting partial changes.
37. Secret-candidate false-positive guard: detection and save-time checks ignore numeric-only and boolean-like policy/toggle values for secret-like keys/attributes (for example action-token lifespan maps and token-claim toggles) while preserving detection for real plaintext secret strings.
38. Metadata selector-path contract: `metadata infer|render|get` accept intermediary selector paths (for example `/admin/realms/_/clients/`), `metadata render` defaults operation by target kind and retries `list` when defaulted `get` is missing a path, infer uses OpenAPI hints when available (including fallback placeholder normalization for non-template-safe OpenAPI parameter names), structured metadata output omits nil directive fields, and infer output omits directives equal to deterministic fallback defaults.
39. Resource get secret-output contract: `resource get` redacts metadata-declared secret attributes to `{{secret .}}` for `--source repository` and `--source remote-server` by default, and `--show-secrets` restores plaintext output for those attributes.
40. CLI completion flag contract: shell completion output avoids duplicate option suggestions that differ only by `=` suffix (for example `--output` and `--output=`).
41. Metadata get output contract: `metadata get` returns resolved metadata overrides merged with default metadata fields by default, `metadata get --overrides-only` returns the compact override object without merged defaults, fallback inference remains endpoint-gated (OpenAPI or remote discovery), and commands still return `NotFoundError` when neither source resolves the endpoint.
42. Metadata infer apply persistence contract: `metadata infer --apply` persists compacted metadata only (no default-equivalent directives), matching the command output contract, and JSON output/persisted JSON metadata end with one trailing newline.
43. Secret get CLI contract: `secret get` accepts path/key positional and flag variants, path-only reads return deterministic `<key>=<value>` text lines, and single-secret reads print raw plaintext values without JSON quoting.
44. Remote collection `NotFound` fallback: `resource get`/`resource save` remote reads treat `404` as an empty list only when collection intent is confirmed by repository structure hints or OpenAPI inference and the collection path is not a misclassified concrete resource path; nested collection reads MUST preserve `NotFound` when the parent resource is also `NotFound`; single-resource paths still use metadata alias/id fallback and preserve `NotFound` when no match exists.
45. Remote read fallback error contract: when single-resource parent-collection fallback receives a non-list validation payload (for example object/array-shape mismatch), commands preserve the original resource `NotFound` instead of surfacing list-decoding validation output.
46. Metadata path indirection contract: rendered operation specs resolve `resourceInfo.collectionPath` templates from handled logical-path context (for example intermediary `/_/` selectors), treat `.`-prefixed operation paths as collection-relative, default omitted operation paths to `.` for `create|list` and `./{{.id}}` for `get|update|delete|compare`, and accept compatibility decoding from `operationInfo.<operation>.url.path`.
47. List jq transform contract: list workflows execute resolved list-operation `jq` expressions before list-shape extraction; valid filters constrain candidate resources deterministically and invalid jq expressions fail with `ValidationError`.
48. Remote metadata singleton fallback contract: when metadata list filtering (`jq`) yields exactly one candidate for a `NotFound` single-resource read, fallback resolves that candidate deterministically (including canonical-ID retry path) only at selector depth and preserves `NotFound` for explicit child identity segments.
49. List jq resource-reference contract: `jq` expressions can call `resource("<logical-path>")`, resolution uses the active workflow source through context resolver wiring, missing resolver context fails with `ValidationError`, repeated lookups are cached per expression evaluation, and invalid arguments/cyclic resolver dependencies fail with `ValidationError`.
50. Resource get explicit collection-marker contract: `resource get <path>/` with remote source resolves `<path>` as a normalized collection list target first; when the list attempt fails with list-shape validation (`list response ...` or `list payload ...`), command flow falls back to one normalized single-resource remote read for `<path>`.
51. Completion space-token contract: shell completion scripts preserve path candidates containing spaces as a single completion token (for example alias segment `AD PRD`) instead of truncating at the first whitespace boundary.
52. Resource-server utility CLI contract: `resource-server get base-url|token-url|access-token` print plaintext values from the active resource-server configuration/auth flow, `get token-url|access-token` fail with `ValidationError` when OAuth2 auth is not configured, and `resource-server check` probes root connectivity while treating `NotFound|Validation|Conflict` probe responses as reachable-success outcomes.
53. Resource-format directive and media-default contract: metadata template rendering supports `{{resource_format .}}` for `json|yaml`, `metadata get` resolves that token in printed metadata while preserving unrelated templates, `resource.json` exact placeholder `{{resource_format .}}` resolves during template/remote workflows without forcing secret resolution, and remote request defaults derive `Accept`/`ContentType` from repository resource format when metadata leaves them unset.
54. E2E component contract validation mode: `run-e2e.sh --validate-components` validates component manifest contract versioning, hook-script syntax, dependency catalog consistency, and resource-server fixture identity metadata (`resourceInfo.idFromAttribute` / `aliasFromAttribute`) before workload execution.
55. Plain-text-only CLI commands (`secret get`, `repo tree`, `resource-server get *`, `resource-server check`, shell `completion` subcommands, and `config print-template`) reject `--output json|yaml` with `ValidationError` instead of silently ignoring structured output requests.
56. CLI exit-code mapping contract: typed errors map to deterministic non-zero process exit codes (`Validation`, `NotFound`, `Auth`, `Conflict`, `Transport`) while untyped/internal errors retain the generic failure code.
57. Repo history CLI contract: `repo history` returns a deterministic not-supported text message for filesystem contexts, supports git-only local history filters (`max-count`, `author`, `grep`, `since`, `until`, repeatable `path`, `reverse`, `oneline`), and forwards parsed filters to the repository history capability.
58. Repo tree CLI contract: `repo tree` prints a deterministic tree-style directory listing, omits files, omits hidden control directories and reserved `_/` metadata directories, and preserves spaces in directory names.
59. Resource list text output contract: `resource list --output text` prints deterministic `<alias> (<id>)` lines, prefers metadata-derived alias/id values, and falls back when metadata identity attributes are absent.
58. Resource mutation inline payload grammar: `resource apply|create|save|update` accept explicit `--payload` forms (file path, `-` stdin, inline JSON/YAML object text, dotted assignments) with negative coverage for malformed inline payloads.
59. Resource git auto-commit contract: `resource save` and repository-backed `resource delete` on git contexts create local commits after successful repository mutation, reject combined `--message` + `--message-override`, and preserve remote-only delete behavior (no repository commit).
60. Resource auto-commit worktree safeguard: `resource save|delete|edit` fail with `ValidationError` before mutation when git worktree has unrelated uncommitted changes.
61. Git repository auto-init contract: git-backed repository status/history/check and git-backed repository mutation commit/status flows initialize a missing local `.git/` repository automatically and continue with normal operation semantics (including empty-history handling for fresh repos).
62. Operation payload validation contract: `resource create|update` and metadata-resolved `resource request post|put|patch` enforce `validate.requiredAttributes`, jq `validate.assertions`, and OpenAPI-backed `validate.schemaRef`; path-derived template fields satisfy required attributes without mutating the transmitted body, and validation failures short-circuit before remote HTTP execution.
63. E2E metadata mode contract: `run-e2e.sh` defaults `--metadata` to `bundle`; `bundle` mode skips component-local `openapi.yaml` wiring and uses shorthand `metadata.bundle` when mapped; `local-dir` mode uses component-local `metadata/` as `metadata.base-dir` and keeps local OpenAPI wiring.

## Failure Modes
1. Tests pass locally with hidden non-determinism.
2. Changed behavior lacks regression coverage.
3. Security-sensitive paths bypass required safeguards.
4. Snapshot/log artifacts leak secret values.

## Edge Cases
1. Suppression removes all comparable fields.
2. Equivalent secret placeholders normalize differently.
3. Non-unique alias appears during apply/refresh.
4. Remote workflow runs with partially configured context.

## Examples
1. Unit test verifies deterministic metadata merge order across wildcard and literal layers.
2. Integration test asserts `ConflictError` when repository sync detects divergence.
3. E2E test validates collection delete safety gates and deterministic summary output.
4. CLI test verifies `metadata render` fallback from defaulted `get` to `list` when `get` has no path and validates completion scripts do not emit duplicate `--flag`/`--flag=` suggestions.
5. CLI test verifies `metadata infer --apply` persists the same compact payload printed to output and that `metadata get` default vs `--overrides-only` output contracts (including endpoint-gated fallback inference) are preserved.
6. CLI + repository tests verify `repo tree` text output is deterministic, rejects structured output, omits files/hidden or reserved directories, and preserves directory names containing spaces.
7. CLI test verifies explicit payload collection-target inference (`resource create /admin/realms --payload realm=test`) resolves the concrete child path from metadata alias/id hints while the equivalent explicit resource path (`/admin/realms/test`) continues to pass identity validation.
8. Bundle metadata tests verify OpenAPI fallback discovery order when `resource-server.http.openapi` is empty: `declarest.openapi` hint, bundle-root/metadata-root OpenAPI files, then deterministic recursive bundle-file fallback.
