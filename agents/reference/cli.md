# CLI Commands, Input Grammar, Output, and Completion

## Purpose
Define the user-facing CLI contract: command tree, flags, input grammar, output stability, exit codes, and completion behavior. Underlying domain rules are owned elsewhere (secrets -> secrets.md; defaults -> metadata.md; identity/required-attrs -> metadata.md + domain.md; read/delete/save/apply fallback strategy -> orchestrator.md + managed-service.md; error taxonomy -> interfaces.md); this file specifies only their CLI-level surface and enforces them at the command boundary.

## Scope
Path ambiguity is resolved by the path-flag-vs-positional rules below. Command-framework internals, shell installation, and transport internals are out of scope.

## Normative Rules

### General CLI Contract
1. Each CLI command MUST map directly to an orchestrator use case.
2. Input validation MUST fail fast with usage guidance and a non-zero exit code.
3. Destructive operations MUST require an explicit confirmation flag (`--yes`/`-y` for deletes, `--force` for overwrites, `--force-push` for non-fast-forward push) or equivalent safety gate.
4. Machine-parsing output MUST be stable and documented; human-readable output SHOULD be concise and deterministic.
5. Command aliases MUST NOT introduce ambiguous behavior.
6. Interactive flows MUST run only when stdin and stdout are interactive terminals and MUST preserve non-interactive automation behavior; an interactive flow invoked without required arguments in a non-interactive environment MUST fail with `ValidationError`.
7. CLI process exit codes SHOULD map typed error categories (validation, not-found, auth, conflict, transport) deterministically instead of collapsing to one code.

### Bootstrap Gating (no active-context resolution required)
8. Help (`--help`, `-h`, `help`) and completion-script invocations (`completion`, `__complete`, `__completeNoDesc`) MUST render without active-context resolution.
9. Invoking a command group without a required subcommand MUST render that group's help without active-context resolution.
10. Non-runtime commands MUST execute without active-context resolution at startup: `version` and `context print-template|add|edit|update|delete|rename|list|use|show|current|clean|session-hook|resolve|validate|check|init`.
11. Runtime commands MUST continue to fail fast when active-context resolution is required.

### Completion
12. Completion suggestions MUST be context-aware and deterministic, expose canonical command names, and MUST NOT leak internal command placeholders or aliases (root completion includes `help`).
13. Path completion MUST merge repository, remote, and OpenAPI paths. For templated OpenAPI segments (`{...}`), completion SHOULD resolve concrete candidates by listing collection children with metadata-aware path semantics, and MUST NOT surface `{...}` placeholders as completion items.
14. Path completion MUST use command-aware source priority: `resource get|save|list|delete` prefer remote (respecting explicit `--source`); `resource apply|create|update|diff|explain|template` prefer repository, falling back to remote only when repository yields no candidates; `resource request <method>` prefers remote with repository fallback; `resource metadata *` and path-aware `secret` commands prefer repository with remote fallback.
15. When resolving collection items from payload-backed metadata, completion MUST prefer rendered `resource.alias` over ID-only segments when available, MUST emit canonical absolute paths that stay prefix-compatible with the current token, SHOULD preserve trailing `/` on collection prefixes, and SHOULD emit no-space directives so accepted path candidates do not append a trailing space.
16. Completion SHOULD surface metadata-defined logical child segments (intermediary `/_/` selector templates such as `/admin/realms/_/user-registry/_/mappers/_/`) as canonical suggestions under matching concrete paths, preserving `_` selector segments verbatim rather than substituting placeholder text.
17. Generated completion scripts MUST preserve a path candidate containing spaces in non-terminal segments (for example `/admin/realms/publico-br/user-registry/AD PRD`) as one completion token.
18. Completion MUST complete the path token at the cursor position even when later arguments remain on the command line (for example `resource get /adm<cursor> --source managed-service`).
19. Completion output SHOULD avoid duplicate flag suggestions differing only by an `=` suffix (for example `--output` and `--output=`).

### Global Flags
20. Global flags: `--context|-c`, `--debug|-d`, `--verbose|-v`, `--skip-result-message|-n`, `--ignore-warnings`, `--no-color`, `--output|-o` (`auto|text|json|yaml`), `--help|-h`.
21. Flag defaults MUST honor precedence `flag > env > built-in default`, using `DECLAREST_CONTEXT`, `DECLAREST_OUTPUT`, `DECLAREST_VERBOSE`, `DECLAREST_VERBOSE_INSECURE`, `DECLAREST_SKIP_RESULT_MESSAGE`, `DECLAREST_IGNORE_WARNINGS`, `DECLAREST_NO_COLOR`, plus `NO_COLOR` (any non-empty value disables color). Readers MUST accept legacy `DECLAREST_NO_STATUS` as an alias for `DECLAREST_SKIP_RESULT_MESSAGE`.

### Path Inputs (path-aware commands)
22. Path-aware commands MUST accept positional `<path>` and `--path|-p`; if both are provided and differ, the command MUST fail with `ValidationError`. Paths are logical absolute paths.

### Payload and Content-Type Inputs
23. `--payload|-f <path|->` reads from a file or, with `-`, from stdin.
24. `--content-type|-i` accepts short-form values only: context-catalog commands accept `json|yaml`; resource payload commands accept `json|yaml|xml|hcl|ini|properties|text|binary`.
25. Resource mutation commands (`apply`, `create`, `update`, `save`) accept explicit payload input as a `--payload` file path, `-` (stdin), inline structured/text object text, or JSON Pointer assignments (`/a=b,/c=d,/e/f/g=h`); explicit input MUST override repository-sourced payload loading. Path-looking `--payload` values (for example `private.key`, `./payload.json`, `dir/payload.key`) MUST be treated as file inputs first and MUST fail when the file is missing rather than being decoded as literal inline payload. Inline text and JSON Pointer assignments MUST NOT be accepted for binary content types.
26. When `--content-type` selects a non-structured format (for example `text`, `txt`, `text/plain`), inline `--payload` MUST be treated as literal content and MUST NOT be parsed as assignment shorthand.
27. `resource request post|put|patch` additionally support an inline `--payload` flag for non-binary formats, decoded per `--content-type` when provided, mutually exclusive with `--payload <path|->`/stdin.
28. `--http-method` (on `resource get|list|apply|create|update|delete`) MUST override the rendered metadata operation `method` for the corresponding remote operation(s).

### Source Flags
29. `resource get|list` MUST support `--source <managed-service|repository>`; `resource delete` MUST support `--source <managed-service|repository|both>`. All three default `--source` to `managed-service`. Invalid `--source` values MUST fail with `ValidationError`.

### When `--repo-type git`
30. When `--repo-type git` is selected and no `--git-provider` is supplied, the CLI MUST default the provider to the local `git` component; an explicit `--git-provider` MUST still override.

## Command Tree

Command groups: `context`, `repository`, `resource`, `server`, `secret`, `completion`, `version`.

`resource` subcommands: `get`, `save`, `apply`, `create`, `update`, `delete`, `diff`, `list`, `explain`, `template`, `edit`, `copy`, `defaults`, `request`, `metadata`.
`resource defaults` subcommands: `get`, `edit`, `config get|edit`, `profile get|edit|delete`, `infer`.
`resource metadata` subcommands: `get`, `edit`, `resolve`, `render`, `infer`.
`resource request <method>` is the canonical HTTP request path; methods: `get|head|options|post|put|patch|delete|trace|connect`.
`context` subcommands: `add`, `init`, `edit`, `update`, `validate`, `use`, `show`, `current`, `rename`, `delete`, `clean`, `session-hook`, `resolve`, `check`, `print-template`, `list`.
`repository` subcommands: `status`, `clean`, `commit`, `history`, `tree`, `push`.
`secret` subcommands: `set`, `get`, `list`, `delete`, `mask`, `resolve`, `normalize`, `detect`.
`server get` subcommands: `base-url`, `token-url`, `access-token`; plus `server check`.

## Resource Read, List, Delete

1. `resource get` default (`--source managed-service`) MUST attempt the literal remote path first, then list/filter by metadata-derived identity on `NotFound`; when metadata path rendering cannot derive required template fields from the logical path alone, it MUST attempt one parent-collection list, match a unique alias/id, and rerender the read with that candidate payload before surfacing the original validation error (fallback strategy owned by orchestrator.md + managed-service.md).
2. `resource get` MUST redact metadata-declared `resource.secretAttributes` using `{{secret .}}` placeholders for both sources by default; `--show-secrets` MUST disable redaction and print plaintext (secret semantics owned by secrets.md).
3. `resource get --show-metadata` MUST include the rendered metadata snapshot (defaults merged with overrides), presenting resolved `operations.*` directives and resolving payload-aware helper tokens such as `{{payload_media_type .}}` against the active payload descriptor.
4. `resource get --prune-defaults` MUST compact the returned payload against resolved metadata defaults for the same logical path (both sources) before output; when every field is defaulted, the payload MUST print as `{}`, not `null` (defaults semantics owned by metadata.md).
5. `resource get` with an explicit trailing-slash collection marker on a remote source MUST execute remote list resolution for the normalized collection path first; on list-shape validation failure (`list response ...` or `list payload ...`) it MUST retry a single-resource remote read for the same normalized path.
6. `resource get|list` MUST support `--exclude <item[,item...]>` to exclude collection items matched by direct child segment, resolved alias, or resolved ID when the result is a collection.
7. `resource list` MUST support `--recursive` (default non-recursive direct-child listing).
8. `resource delete` MUST support `--recursive` (default non-recursive). `resource delete --source managed-service` MUST resolve collection targets from local repository resources (direct-child by default, descendants with `--recursive`) and, when no local targets match, attempt literal delete with metadata-aware remote identity fallback on `NotFound`, including the unique parent-collection candidate fallback described in rule 1 before surfacing the original validation error.

## Resource Apply, Create, Update

9. `resource apply` MUST treat collection paths as batch targets resolved from local repository resources, default to non-recursive direct-child execution when payload input is absent, and include descendants with `--recursive`.
10. `resource apply` with explicit payload input MUST target a single path, use explicit input instead of repository payloads, read remote state first, create only on `NotFound`, compare desired vs remote with metadata compare directives, and update only when drift exists unless `--force`. `--force` MUST execute the update even when compare indicates no drift. `--recursive` with explicit payload input MUST be rejected.
11. `resource create` with explicit payload input MUST perform a single remote mutation; without payload input it MUST load local repository payloads for resources under the target path and create each resolved target.
12. `resource update` with explicit payload input MUST perform a single remote mutation; without payload input it MUST load local repository payloads and update each matching resource (direct-child only without `--recursive`, descendants with `--recursive`).
13. `resource apply|create|update` collection-target resolution MUST attempt a non-recursive collection list first and, when no entries match a deep path target, attempt single-resource fallback lookup before returning `NotFound`. Repository-backed single-resource reads (`get --source repository`, `apply`, `update`, `diff`, `explain`) MUST attempt literal repository lookup first and, on `NotFound`, perform a bounded collection fallback matching by metadata `resource.id`, using reverse matching only for a simple single-pointer template (fallback owned by orchestrator.md).
14. `resource apply|create|update` explicit-input mode MUST accept a collection target path when metadata exposes a wildcard child and the payload contains metadata-derived alias/id values, inferring one concrete child logical path from the payload identity before remote mutation. Explicit resource-path inputs MUST preserve identity-match validation, and `create|apply` MUST fail with `ValidationError` when the metadata-rendered identity (`resource.alias`/`resource.id`) from the payload does not match the target path segment (identity semantics owned by metadata.md + domain.md).

## Resource Save

15. `resource save` without payload input MUST read the requested path from remote and persist it, using the same literal-then-list/filter metadata-aware fallback as `resource get`.
16. `resource save` MUST support optional explicit payload input (`--payload` file path, `-`/stdin, inline structured object text, or JSON Pointer assignments) and `--mode <auto|items|single>`. `--mode auto` MUST save non-list payloads as one resource and fan out list payloads (`[]` or objects with `items` arrays) into one repository resource per resolved item. When a list item lacks the metadata-defined alias/id attributes, item identity MUST fall back to common attributes (`clientId`, `id`, `name`, `alias`) before failing alias resolution, and per-item payload descriptors MUST be preserved.
17. `resource save` MUST support `--exclude <item[,item...]>` for collection saves (excluding children from managed-service reads and list-item saves before persistence); `--exclude` with `--mode single` MUST fail with `ValidationError`.
18. `resource save --secret` MUST store the entire encoded payload in the secret store under key `<logical-path>:.`, persist only an exact root `{{secret .}}` placeholder using the original descriptor and file suffix, persist metadata `resource.secret: true`, behave as a single-resource save even for list payloads, and reject `--mode items`, `--exclude`, `--secret-attributes`, and `--allow-plaintext` (secret storage owned by secrets.md).
19. `resource save` MUST auto-store and mask detected plaintext candidates declared by metadata `resource.secretAttributes` before persistence; non-metadata-declared candidates MUST fail with `ValidationError` unless `--allow-plaintext` or `--secret-attributes` is set; overwriting an existing repository resource MUST additionally require `--force`.
20. `resource save` MUST auto-use whole-resource secret storage for single-resource saves when metadata declares `resource.secret: true` and `--secret-attributes` is not selected.
21. `resource save --secret-attributes` MUST accept an optional comma-separated attribute list, require structured payloads (`json|yaml`), reject non-structured payloads with guidance toward `--secret`, detect plaintext secret attributes, store handled values under path-scoped keys, replace handled values with `{{secret .}}`, and merge handled JSON Pointers into metadata `resource.secretAttributes`. When it handles only a subset, it MUST fail with the plaintext-secret warning listing only unhandled non-metadata-declared candidates unless `--allow-plaintext` is set. Any requested attribute not detected MUST fail with `ValidationError`.
22. `resource save --prune-defaults` MUST compact the fetched or explicit payload against resolved metadata defaults before persistence; for list saves, pruning MUST apply per resolved item path.
23. `resource save` MUST accept `_` as a wildcard path segment only when no payload input is provided, expanding each wildcard level through remote direct-child list lookups before saving; wildcard expansion for resource targets MUST skip unresolved concrete `NotFound` reads and return `NotFoundError` only when no concrete target resolves. Wildcard path with payload input MUST fail with `ValidationError`.
24. For collection list saves (`--mode auto` on list payloads or explicit `--mode items`), plaintext-secret candidate detection MUST be computed once per save from the collection payload set and applied consistently across all items.
25. Remote-workflow payload placeholder resolution MUST resolve attribute-scoped `{{secret .}}` as `<logical-path>:<json-pointer>`, an exact whole-resource `{{secret .}}` as `<logical-path>:.`, and `{{secret <custom-key>}}` as `<logical-path>:<custom-key>` (owned by secrets.md).

## Resource Edit and Copy

26. `resource edit` MUST resolve from the local repository first (same literal-then-bounded metadata-aware fallback as other repository-backed single-resource workflows); on `NotFound` it MUST fall back to one remote read before opening the editor, and persist the edited payload only when decoding and save validation succeed. On git repository contexts it MUST commit changes and MAY autoSync when the git context enables repository autoSync. It MUST reject resources whose resolved payload type is `octet-stream` with `ValidationError`.
27. `resource copy` MUST support positional `[path] [target-path]` and `--path`/`--target-path`; mismatched positional/flag target values MUST fail with `ValidationError`. It MUST read the source from the local repository first and, on `NotFoundError`, retry the source read from remote before applying overrides and save validation. `--override-attributes` MUST accept JSON Pointer assignments and apply them to object payloads before save validation.

## Resource Diff

28. `resource diff` MUST resolve collection targets from local repository resources (direct-child by default, descendants with `--recursive`), compare each resolved resource, and on a deep path with no collection match attempt single-resource fallback before `NotFound`. `--list` MUST emit only changed/added/removed resource paths in stable order. `--color <auto|always|never>` MUST control ANSI rendering (`auto` colors only terminals, `always` forces ANSI, `never` disables).

## Resource Metadata

Metadata structure, layering, rendering, inference, and defaults semantics are owned by metadata.md; the rules below cover CLI surface only.

29. Metadata targets accept collection and resource scopes via positional path and `--path`, including intermediary namespace segments (for example `/admin/realms/_/clients/`) for `get|infer|render`.
30. `resource metadata render` MUST accept an optional operation; when omitted it defaults to `list` for collection/selector targets and `get` for resource targets, and when the defaulted `get` operation path is missing it MUST retry with `list` before returning a validation error.
31. `resource metadata get` MUST return resolved repository metadata in the full canonical nested schema by default, filling unset attributes with deterministic defaults (empty strings, `false`, empty arrays/maps, default operation entries, `null` for unset operation bodies) and preserving helper placeholders such as `{{payload_media_type .}}`; `--overrides-only` MUST return only the resolved/inferred override object without expanded defaults. When overrides are missing, `get` MUST return inferred metadata (compact in `--overrides-only`, default-merged otherwise) if the target endpoint exists in OpenAPI or is reachable; otherwise it MUST keep `NotFoundError`.
32. `resource metadata infer` MUST use OpenAPI path hints when available and still return deterministic fallback inference otherwise, MUST omit inferred directives equal to deterministic fallback defaults, and MUST expose only supported inference options (no placeholder flags for unsupported recursion). `--apply` MUST persist the same compacted payload shown in output; when JSON is selected, both output and persisted JSON MUST end with one trailing newline.
33. `resource metadata edit` MUST open the current override in YAML (starting from an empty metadata object when none exists), validate on save/exit, and persist only validated changes.
34. Stdin mutations MUST validate payload format before side effects; option conflicts MUST produce usage errors.

## Resource Defaults

Defaults model (mode/profiles/includes, defaults-artifact layout) is owned by metadata.md; the rules below cover CLI surface only.

35. `resource defaults get <path>` MUST print the effective resolved defaults object and print `{}` instead of `NotFound` when none are resolved.
36. `resource defaults edit <path>` MUST edit only the local baseline defaults for the exact scope, persisting by default to a selector-local file (`defaults.yaml`/`defaults.properties`) in the active writable metadata target (explicit `metadata.baseDir` when configured, otherwise the repo-local metadata tree) and wiring metadata through the exact placeholder `{{include defaults.<ext>}}`.
37. `resource defaults config get <path>` MUST print only the local persisted `resource.defaults` block for the exact scope, preserving raw include placeholders. `resource defaults config edit <path>` MUST edit only that block (`mode`, `useProfiles`, raw `value`, raw `profiles`) and preserve existing include placeholders.
38. `resource defaults profile get <path> <profile>` MUST print the effective resolved object for that profile. `resource defaults profile edit <path> <profile>` MUST edit only the local profile object for the exact scope, persisting by default to `defaults-<profile>.<ext>` plus the exact placeholder `{{include defaults-<profile>.<ext>}}`. `resource defaults profile delete <path> <profile>` MUST remove only the local profile entry and delete the selector-local file only when the metadata entry points to the deterministic auto-managed filename for that profile.
39. `resource defaults infer <path>` MUST accept one logical collection path or one concrete repository resource path, treat trailing-`/` and non-trailing collection inputs as the same target, return the resolved collection scope even when a concrete resource path is the sample, support `--from <repository|managed-service[,...]>` (default `repository`), and support `--items <alias[,alias...]>` to restrict inference (failing with `ValidationError` when any alias does not exist).
40. `--from repository` MUST infer from direct local child resources under the resolved collection; `--from managed-service` MUST create two temporary remote probe resources per selected local item, infer from observed probe outputs only, clean up all probes before returning, and build each probe payload from only the effective create-required attributes (`operations.create.validate.requiredAttributes` when explicitly set, otherwise the effective `resource.requiredAttributes` create set), always including every JSON Pointer referenced by `resource.alias`.
41. `resource defaults infer --save` MUST persist inferred baseline defaults into the target collection selector directory as `defaults.<ext>` (for example `<collection-path>/_/defaults.yaml`) in the active writable metadata target, preserve an existing defaults-artifact codec, otherwise prefer the collection's effective defaults-capable payload type, fall back to JSON or YAML only when no supported codec resolves, suppress stdout on success, and write into `metadata.baseDir` when configured or the repo-local overlay when the active shared source is `metadata.bundle|bundleFile`.
42. `resource defaults infer --check` MUST print the inferred object and fail with `ValidationError` when it does not match the current resolved defaults for the scope; `--save` and `--check` combined MUST be rejected.
43. When `--from` includes `managed-service`, the command MUST require explicit confirmation before any remote mutation: `--yes` skips prompting; an interactive terminal MAY satisfy it via one prompt warning that temporary remote resources will be created and deleted; non-interactive execution without `--yes` MUST fail with `ValidationError`.
44. `resource defaults infer --wait <duration|seconds>` MUST wait the non-negative interval after creating probes and before the first probe readback (bare integers are seconds; invalid/negative values fail with `ValidationError`); `--wait` without `--from managed-service` MUST fail with `ValidationError`.

## Resource Request

45. `resource request <method>` MUST accept endpoint path from positional `<path>` and `--path` (mismatch -> `ValidationError`). `request get` MUST attempt metadata-aware remote read fallback on `NotFound`, and `request get|delete` MUST reuse the same unique parent-collection candidate fallback as `resource get|delete` when path rendering cannot derive required template fields.
46. `resource request <method>` MUST accept optional payload from `--payload <path|->` or stdin, decoding by `--content-type` when provided, else by trusted file extension, else by content heuristics (`JSON` for structured-looking input, `application/octet-stream` otherwise); binary input MUST be read only from file or stdin and produce `resource.BinaryValue`. (Inline `--payload` for `post|put|patch` per rule 27.)
47. `resource request <method>` MUST support optional `--accept-type` and `--content-type` overrides; when omitted, payload-type-aware defaults MAY be resolved from metadata, request content type, or response headers.
48. `resource request delete` MUST require `--yes` and fail with `ValidationError` when confirmation is not explicit. It MUST resolve collection targets from local repository resources (direct-child by default, descendants with `--recursive`) and issue one delete per resolved target; when no local targets match it MUST issue a single delete request for the requested path.

## Context Commands

Context catalog schema is owned by context-config.md; the rules below cover CLI surface only.

49. `context add` MUST accept input from `--payload <path|->` or stdin as either one `context` object or one full `contexts.yaml` catalog; it MUST accept `--content-type <json|yaml>` for stdin/extension-less input, infer JSON/YAML from a payload file extension when present, and otherwise default extension-less decoding to JSON.
50. `context add` catalog import: with `--context-name` omitted it MUST import all contexts; with `--context-name` set it MUST import only the matching name (`ValidationError` when no catalog context matches). For a single-context input with `--context-name` set, the imported name MUST be overridden by `--context-name`. It MUST also accept a context name from positional `[new-context-name]` or global `--context`, failing with `ValidationError` when both are provided and differ.
51. `context add --set-current` MUST set current to the imported context when exactly one is imported; with multiple imported contexts it MUST require catalog `currentContext` or fail with `ValidationError`.
52. Interactive `context add` (no file/stdin input) MUST support full context-schema authoring: prompt required repository and managed-service fields, offer skip paths for optional sections (for example `secret-store`, `preferences`), and enforce one-of branching (for example oauth2 vs basic-auth vs custom-headers) by collecting only the selected option's fields. It MUST treat `managed-service` as required, MUST NOT prompt for repository payload format (managed-service media signals and explicit payload input determine `resource.<ext>` persistence at runtime), and MUST skip name prompting when a name is supplied via positional or `--context`.
53. `context add|edit|update|validate` MUST fail validation when `managed-service` is omitted. `context update` and `context validate` SHOULD follow the same `--content-type` plus file-extension decoding rules as `context add`.
54. `context use`, `context show`, `context rename`, and `context delete` SHOULD support interactive selection (and rename target-name prompt, delete confirmation) when arguments are omitted.
55. `context show` MUST accept optional selection from positional `[name]` or global `--context` (mismatch -> `ValidationError`); when neither is provided it MUST require interactive selection.
56. `context resolve`, `context check`, and `context init` MUST accept optional selection from positional `[name]` or global `--context` (mismatch -> `ValidationError`). `context init` MUST initialize repository state and resolve metadata at `/` so bundle-backed references are downloaded and cached before runtime workflows.
57. `context edit` MUST open the catalog in an editor, validate the edited YAML on save/exit, and persist only validated changes. `context edit <name>` MUST present only the selected context plus catalog-scoped `defaultEditor` and reusable `credentials`, and MUST merge the validated result back into the full catalog without exposing unrelated contexts.
58. `context clean` MUST require at least one cleanup selector flag. `context clean --credentials-in-session` MUST remove prompt-backed credential session cache files for the detected prompt-auth session so later commands prompt again, and MUST succeed without a current context.
59. `context session-hook <bash|zsh>` MUST print text-only shell code that exports `DECLAREST_PROMPT_AUTH_SESSION_ID` once per session, registers `context clean --credentials-in-session` cleanup on shell exit, preserves pre-existing exit handlers, and MUST succeed without a current context.
60. `context print-template` MUST accept no positional arguments, work without a current context, and output a commented `contexts.yaml` template covering all configuration branches with mutually-exclusive blocks explicitly marked.

## Repository Commands

61. `repository push` MUST support `--force-push` for non-fast-forward push intent and MUST fail with `ValidationError` when the active repository type is `filesystem` or when type is `git` without `repository.git.remote` configuration.
62. `repository clean` MUST discard uncommitted tracked and untracked changes for git repositories and succeed as a no-op for filesystem repositories.
63. `repository commit` MUST accept `--message|-m`, fail with `ValidationError` for `filesystem` repositories, create at most one local commit from current worktree changes, and on a clean worktree succeed as a no-op reporting that no commit was created.
64. `repository status --verbose` (global `--verbose`) MUST include deterministic local worktree change details for git repositories.
65. `repository history` MUST return a deterministic not-supported message for filesystem repositories and expose filtered local git history (for example `--oneline`, `--max-count`, `--author`, `--grep`, `--path`) for git repositories.
66. `repository tree` MUST accept no positional arguments and print a deterministic directory-only tree of the local repository, excluding files, hidden control directories (for example `.git`), and reserved metadata namespace directories named `_`; directory names with spaces MUST be preserved verbatim.

## Secret Commands

Secret lifecycle, detection, masking, and key mapping are owned by secrets.md; the rules below cover CLI surface only.

67. `secret set` MUST accept `secret set <key> <value>`, `secret set <path> <key> <value>`, `secret set --path <path> --key <key> <value>`, and `secret set <path>:<key> <value>`.
68. `secret get` MUST accept `secret get <key>` (direct key), `secret get <path> <pointer>`, `secret get --path <path> --key <pointer>`, and `secret get <path>:<pointer>`. `secret get <path>`/`--path <path>` without an explicit key MUST fail with `ValidationError` directing the user to `secret list`. `secret delete` MUST accept the same single-secret target grammar.
69. `secret get|set|delete --key` MUST require `--path`.
70. `secret list` MUST accept optional path selection (positional `<path>` or `--path`, treated as logical absolute) and optional `--recursive`, return keys only (never plaintext) in deterministic order: without a path it returns all keys; `secret list <path>` without `--recursive` returns only keys stored exactly at `<path>` rendered relative to it; with `--recursive` it returns those keys plus descendant path-scoped keys rendered as the full relative path from the selected root (for example `/test/secrets/private-key:.`).
71. `secret detect` without input payload MUST scan local repository resources recursively under positional `<path>`/`--path` (default `/`). `--fix` MUST persist detected attributes into metadata `resource.secretAttributes`: in input-payload mode it MUST require a target path from positional `<path>` or `--path`; in repository-scan mode it MUST merge detected attributes for each detected resource path in scope. `--secret-attribute <pointer>` MUST apply only that detected pointer and fail with `ValidationError` when it is not detected in payload or repository scope.

## Server Commands

72. `server get base-url` MUST print the active context `managedService.http.url` and fail with `ValidationError` when `managedService.http` is not configured.
73. `server get token-url` MUST print `managedService.http.auth.oauth2.tokenURL`, and `server get access-token` MUST fetch and print the OAuth2 access token; both MUST fail with `ValidationError` when OAuth2 auth is not configured.
74. `server check` MUST probe managed-service connectivity with a GET against `managedService.http.healthCheck` when configured (otherwise the normalized `managedService.http.url` path) and succeed only when the probe succeeds.

## Editors and Git Auto-Behavior

75. Editor-opening commands (`context edit`, `resource edit`) MUST support `--editor <command>` to override the catalog `default-editor` and the built-in `vi` fallback.
76. `resource save` on a git context MUST create a local commit after repository mutation and accept `--message` as an override-only commit message; `--push` MUST push the resulting commit regardless of `repository.git.remote.autoSync` and fail with `ValidationError` when the repository is not git or has no configured `repository.git.remote`.
77. `resource delete` with repository deletion selected (`--source repository|both`) on a git context MUST create a local commit after mutation and accept the same `--message` flag with the same override-only rule.
78. Auto-commit-enabled mutation commands (`resource save|delete|edit`) MUST require a clean git worktree before mutation. A `--message` value that is empty or whitespace-only MUST fail.
79. Git-backed repository command flows and mutation post-actions (for example `repository status|clean|history|check|refresh|reset|push` and resource auto-commit/status checks) MUST auto-initialize the local git repository when `.git/` is missing before continuing.

## Output Contract

1. Structured (`--output json|yaml`) output MUST define stable keys for automation; error output MUST include category and an actionable next step where possible.
2. `--output auto` (default): single-resource payloads follow the resolved payload type (structured render when supported, text payloads print raw text, binary payloads write raw bytes with no trailing newline); `resource list` defaults to text rendering. `--output auto|text` MUST reject collection/multi-item results containing binary payloads with `ValidationError`.
3. Plain-text-only commands MUST reject `--output json|yaml` with `ValidationError`: `secret get`, `repository tree`, `server get *`, `server check`, shell `completion` subcommands, and `context print-template`.
4. `context show` MUST print YAML by default and reject `--output json`; it MAY accept `--output text|yaml`. Its output MUST be a valid one-context catalog view to stdout, preserving catalog-scoped attributes (`credentials`, `defaultEditor`) and explicit context fields without view-time compaction, and setting `currentContext` to the shown context name.
5. `resource list --output auto|text` MUST render one line per item as `<alias> (<id>)`, preferring metadata-derived identity and falling back to resolved/logical-path identity when absent; text output SHOULD align the alias column deterministically.
6. `resource diff --output auto|text` MUST render normalized unified-diff text against compare-transformed payloads: one highlighted section for a single target, or one section per changed/added/removed resource plus a deterministic summary line for multi-resource targets, omitting unchanged resources by default. Text-mode binary diff MUST render a deterministic whole-payload "binary content differs" message instead of field-level rendering. `--list` MUST print one path per line for `auto|text` and a stable list of strings for `json|yaml`.
7. Structured `resource diff` and `resource explain` MUST encode `resource.DiffEntry.ResourcePath` as the logical resource path and `resource.DiffEntry.Path` as an RFC 6901 JSON Pointer relative to that payload; root replacements MUST use `Path=""`.
8. Structured `--output json|yaml` for binary payloads MUST emit a stable wrapper with `encoding=base64`, `mediaType=application/octet-stream`, and `data`.
9. Metadata structured output MUST keep compact omit-empty semantics for `resource metadata resolve|infer` and `resource metadata get --overrides-only`, while default `resource metadata get` emits the full canonical nested shape with explicit defaults; metadata JSON output and persisted JSON from `infer --apply` MUST end with one trailing newline.
10. `repository status --output auto` MUST render a deterministic text summary; `--verbose` text MUST append git-style short worktree detail lines for git repositories and print `worktree=clean` when no changes exist, and MAY include structured `worktree` entries under `--output json|yaml`. `repository status` text MUST be repository-type aware: `filesystem` reports `sync=not_applicable`; `git` reports git sync state with `remote=not_configured` when remote config is absent.
11. `repository commit` MUST support `--output text|json|yaml`; `json|yaml` MUST expose a stable `committed` indicator, and text MUST deterministically report whether a commit was created (including the clean-worktree no-op). Text success SHOULD use the standard execution-status footer.
12. `repository tree` text output MUST render a deterministic tree-style listing using repository-relative directory names only (no files), preserving spaces within segments.
13. `context check` text output MUST report component rows labeled `context`, `repository`, `metadata`, `managed-service`, and `secret-store`.
14. `secret get` output MUST always be plain text: single-secret reads print only the value line; path reads print one `<key>=<value>` line per matched secret without JSON quoting, preserving quote characters only when present in values.
15. `server get base-url|token-url|access-token` output MUST be plain text, printing only the requested value followed by one trailing newline; `server check` text MUST print a concise probe result line on success.
16. State-changing commands (`resource save|apply|create|update|delete` and `resource request post|put|patch|delete|connect`) MUST suppress complementary payload output by default and print only the status footer; `--verbose` MUST re-enable that payload output. Commands returning a nil payload MUST emit no payload body (no `null`/`<nil>` placeholder).
17. Unless `--skip-result-message` is set, the above state-changing commands MUST print a terminal status line as the final stderr output: `[OK] <description>.` on success, `[ERROR] <description>.` on failure (interactive terminals render `[OK]` bold green, `[ERROR]` bold red).
18. Standalone stderr warning lines SHOULD use `[WARNING] <description>` (interactive terminals render bold yellow), preserving `--no-color`/`NO_COLOR`; `--ignore-warnings` MUST suppress them entirely. Warnings emitted before a successful footer MUST preserve deterministic order and still allow the final `[OK]` line to remain the last stderr line for mutation commands.
19. The CLI MUST support `--no-color` and the `NO_COLOR` environment variable to disable ANSI color for status labels.
20. HTTP transport debug output MUST include TLS/mTLS context (`tls_enabled`, `mtls_enabled`, configured TLS file paths) without logging secret values.
21. Help output MUST present `--help` in the `Global Flags` section and SHOULD avoid repeated blank lines between sections.

## Failure Modes

1. Missing required path argument; positional path and `--path` differ; `resource copy` positional/flag target-path differ.
2. Invalid payload format; unsupported command/flag combination; required manager not present in active context.
3. `resource get|list|delete` invalid `--source`; `resource save --mode` not `auto|items|single`; `resource save --mode items` on non-list input.
4. `resource save` plaintext-secret guards: non-metadata-declared candidate without `--allow-plaintext`/`--secret-attributes`; metadata-declared plaintext or whole-resource candidate without a configured secret provider; subset-only `--secret-attributes` leaving unhandled non-declared candidates without `--allow-plaintext`; overwriting an existing repository resource without `--force`.
5. `resource save --secret-attributes` undetected attribute, or used for non-structured payload; `resource save --secret` combined with `--mode items|--exclude|--secret-attributes|--allow-plaintext`; `resource save --exclude` with `--mode single`.
6. `resource apply|create|update|save` path-looking `--payload` that does not resolve to a file; binary content type receiving inline `--payload` text or JSON Pointer assignments.
7. `resource create` without payload input and no matching local resources; `resource apply|create|update|diff` on a collection with no local resources.
8. `resource create|apply` explicit-input identity (`resource.alias`/`resource.id`) does not match target path segment; metadata-aware identity fallback yields multiple candidates -> `ConflictError`.
9. `resource save` wildcard path combined with payload input.
10. `resource request post|put|patch` inline `--payload` combined with `--payload <path|->`/stdin; `resource request delete` without `--yes`.
11. `resource edit` target resolves to `octet-stream`; `--output auto|text` on a collection/multi-item result containing binary payloads.
12. `secret detect --fix` with payload input but no path input; `secret detect --secret-attribute` value not detected; `secret get --key` without `--path`; `secret get <path>:` empty key segment.
13. `context add` positional name and `--context` differ; `context add --context-name` matches no catalog context; `context add --set-current` with multiple imported contexts and missing catalog `currentContext`; context-catalog mutation omits `managed-service`; `context print-template` with positional arguments; `context clean` without any cleanup selector flag.
14. `repository push` for a `filesystem` context (or `git` without remote config); `resource save|delete` auto-commit while the git worktree has unrelated uncommitted changes; `resource save|delete|copy` `--message` empty or whitespace-only; `resource save --push` on filesystem or without git remote config.
15. `server get token-url|access-token` when managed-service auth is not OAuth2.

## Examples

1. `declarest resource apply /customers --recursive` applies direct and nested local resources; `declarest resource apply /customers/acme --payload payload.json --force` applies one explicit payload, forcing update even with no drift (and `--recursive` with explicit payload is rejected).
2. `cat payload.json | declarest resource create /customers/acme --payload -` creates one remote resource from stdin, overriding repository-sourced input; `declarest resource update /customers` updates only direct-child repository resources, skipping descendants.
3. `declarest resource get /customers/acme --source repository --prune-defaults` prints only non-default override fields from the local payload; `declarest resource get /admin/realms/ --exclude master,realm1` returns remaining realm payloads.
4. `declarest resource get /admin/realms/master/` first attempts a remote list for `/admin/realms/master`, then falls back to one single-resource read when the list response shape is invalid.
5. `declarest resource get /files/blob --output text` writes raw binary bytes with no trailing newline; `--output json` emits the base64 `{encoding,mediaType,data}` wrapper.
6. `declarest resource list /customers --output text` prints one `<alias> (<id>)` line per item, falling back to logical-path alias formatting when identity attributes are absent.
7. `declarest resource delete /customers/acme --yes --source both` deletes from remote and repository; `declarest resource delete /admin/realms/master/clients/account --yes` retries deletion via metadata-resolved remote ID when the literal delete path is not found.
8. `declarest resource save /customers < list.json` stores each list item as its own resource; `--mode single` stores the whole list payload in one file.
9. `declarest resource save /customers/acme < payload.json` fails when undeclared plaintext secret candidates are detected; with metadata `resource.secretAttributes: [/credentials/authValue]` that attribute is stored and masked automatically; `--secret-attributes=/password` handles only `/password` and fails listing remaining unhandled candidates unless `--allow-plaintext`.
10. `declarest resource save /projects/platform/secrets/private-key --payload private.key --secret --force` stores the `.key` payload under `/projects/platform/secrets/private-key:.`, persists `resource.secret: true`, and writes only `{{secret .}}` to `resource.key`; a missing-file `--payload` token fails instead of being saved as literal content.
11. `declarest resource save /projects/test/secrets/pass-word --payload a=b --content-type txt --secret --force` keeps `a=b` as literal text; `declarest --context git resource save /customers/acme --payload payload.json --force --push --message ticket-123` saves, commits with message `ticket-123`, and pushes even when `autoSync` is disabled.
12. `declarest resource save /admin/realms/_/clients/test` expands wildcard realms and saves each matched `test` client, returning `NotFoundError` only when no realm contains a match.
13. `declarest resource diff /customers --recursive --list` prints only differing descendant paths in stable order; `declarest resource diff /admin/realms/payments --color always` forces a colored diff even when stdout is not a terminal.
14. `declarest resource request post /customers --payload '{"id":"acme"}'` sends an inline JSON body; `declarest resource request put /files/blob --payload blob.bin --content-type binary` uploads binary; `declarest resource request delete /customers/acme` fails (needs `--yes`), `--yes --recursive` issues one delete per resolved repository target.
15. `declarest resource metadata render /admin/realms/_/clients/` defaults to rendering the `list` operation for the selector collection; `declarest resource metadata infer /admin/realms/_/clients/` uses OpenAPI hints when available.
16. `declarest resource defaults infer /admin/realms --from managed-service --save --yes --ignore-warnings` infers defaults from temporary remote probes, suppresses bootstrap `[WARNING]` lines, and still succeeds; `--save` and `--check` together are rejected.
17. `declarest secret list /projects --recursive` prints relative descendant entries such as `/test/secrets/private-key:.`; `declarest secret get /customers/acme` fails and points to `declarest secret list /customers/acme`.
18. `declarest context add --payload contexts.yaml --context-name prod --set-current` imports only `prod` and sets it current; `--set-current` with multiple imported contexts fails when the catalog omits `currentContext`; bare `declarest context add` opens interactive prompts and never asks for repository payload format.
19. `declarest context check prod --context dev` fails with `ValidationError` (positional/flag selection conflict); `declarest context show --context dev` prints a valid one-context catalog including shared credentials.
20. `declarest repository status` in a filesystem context prints `type=filesystem sync=not_applicable hasUncommitted=<bool>`; `declarest repository push` there fails with `ValidationError`; `declarest repository tree` preserves spaces in directory names (for example `AD PRD`) and omits files.
21. `declarest resource get /admin/realms/m<TAB>` completes to concrete candidates such as `/admin/realms/master`; `/admin/realms/master/clients/<TAB>` uses `resource.alias` values (for example `account`) over ID segments; `/admin/realms/publico-br/user-registry/A<TAB>` can complete to `/admin/realms/publico-br/user-registry/AD PRD` as one token; on remote lookup failure, completion falls back to repository candidates.
22. `declarest resource save --help` and `declarest completion bash` succeed with no current context; `declarest` root tab completion suggests `help` and no internal helper names.
23. `declarest server get access-token` prints only the OAuth2 access token; it (and `server get token-url`) fail with `ValidationError` for `basic-auth`/`custom-headers` contexts; `declarest server check` reports a successful probe only when the configured GET succeeds.
