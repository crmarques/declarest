# CLI Commands, Input Grammar, Output, and Completion

## Purpose
Define user-facing CLI contract, command semantics, output stability, and completion behavior.

## In Scope
1. Command groups and operation semantics.
2. Input grammar and argument validation.
3. Output and error formatting contracts.
4. Completion behavior for paths and resources.

## Out of Scope
1. Internal command framework implementation details.
2. Shell-specific installation details.
3. Transport protocol internals.

## Normative Rules
1. CLI commands MUST map directly to orchestrator use cases.
2. Input validation MUST fail fast with usage guidance and non-zero exit codes.
3. Destructive operations MUST require explicit confirmation flag or equivalent safety gate.
4. Output for machine parsing MUST be stable and documented.
5. Human-readable output SHOULD be concise and deterministic.
6. Command aliases MUST not introduce ambiguous behavior.
7. Completion suggestions MUST be context-aware and deterministic.
8. `resource request <method>` SHOULD be the canonical HTTP request command path.
9. Interactive command flows MUST only run when stdin/stdout are interactive terminals and MUST preserve non-interactive automation behavior.
10. Help invocations (`--help`, `-h`, or `help`) and completion-script invocations (`completion`, `__complete`, `__completeNoDesc`) MUST render without requiring active-context resolution.
11. Shell completion output MUST expose canonical command names and MUST NOT leak internal command placeholders.
12. Invoking a command group without a required subcommand MUST render that group's help and MUST NOT require active-context resolution.
13. Non-runtime commands (`version`, `config print-template|add|edit|update|delete|rename|list|use|show|current|resolve|validate`) MUST execute without requiring active-context resolution at startup.
14. When `--repo-type git` is selected and no `--git-provider` is supplied, the CLI MUST default the provider to the local `git` component so git-backed repositories integrate without additional flags while still enforcing explicit overrides when provided.
15. Path completion MUST merge repository paths, remote resource paths, and OpenAPI paths; for templated OpenAPI segments (`{...}`), completion SHOULD resolve concrete candidates by listing collection children with metadata-aware path semantics.
16. Path completion MUST use command-aware source priority: `resource get|save|list|delete` MUST prefer remote candidates by default (respecting explicit source flags), repository-driven commands (`resource apply|create|update|diff|explain|template`) MUST prefer repository candidates and only fall back to remote candidates when the preferred source yields no completion candidates, `resource request <method>` path completion MUST prefer remote candidates with repository fallback, and `metadata *` plus path-aware `secret` commands MUST prefer repository candidates with remote fallback.
17. When completion resolves collection items from payload-backed metadata, it MUST prefer `aliasFromAttribute` values for displayed path segments over ID-only segments when aliases are available, completion suggestions MUST use canonical absolute paths that remain prefix-compatible with the current input token, templated placeholder segments (`{...}`) MUST NOT be surfaced as completion items, collection-prefix suggestions SHOULD preserve trailing `/` semantics, and path completion SHOULD emit no-space directives so accepted path candidates do not append a trailing space.
18. When metadata selector trees define logical child branches that are not present in OpenAPI paths (for example intermediary `/_/` templates such as `/admin/realms/_/user-registry/_/mappers/_/`), path completion SHOULD surface those metadata-defined child segments as canonical logical suggestions under matching concrete paths.

## Data Contracts
Command groups:
1. Basic Commands: `config`, `metadata`, `repo`, `resource`, `resource-server`, `secret`.
2. Other Commands: `completion`, `version`.
Global flags:
1. `--context`, `-c`.
2. `--debug`, `-d`.
3. `--verbose`, `-v`.
4. `--no-status`, `-n`.
5. `--no-color`.
6. `--output`, `-o` with allowed formats `auto|text|json|yaml`.
7. `--help`, `-h`.

Input flags:
1. `--payload <path|->`, `-f` (use `-` to read object from stdin).
2. `--format`, `-i` with allowed formats `json|yaml`.
3. `--payload` as a command-specific inline payload flag for `resource request post` and `resource request put`; resource mutation commands (`apply`, `create`, `update`, `save`) accept explicit payload input from `--payload` as a file path, `-` (stdin), inline JSON/YAML object text, or dotted assignments (`a=b,c=d,e.f.g=h`), and explicit input MUST override repository-sourced payload loading when provided.
4. `--http-method` as a command-specific metadata operation HTTP-method override flag for `resource get|list|apply|create|update|delete`; when provided, it MUST override the rendered metadata operation `method` for the corresponding remote operation(s).
5. `resource save` SHOULD use `--overwrite` for local overwrite confirmation (legacy hidden alias `--force` MAY remain during migrations).
6. `resource copy` SHOULD use `--overwrite` for local overwrite confirmation (legacy hidden alias `--override` MAY remain during migrations).
7. `resource delete` and `resource request delete` SHOULD use `--confirm-delete`, `-y` for destructive confirmation (legacy hidden alias `--force` MAY remain during migrations).
8. `repo push` SHOULD use `--force-push`, `-y` for non-fast-forward push intent (legacy hidden alias `--force` MAY remain during migrations).

Path flags:
1. Path-aware commands MUST accept `--path`, `-p`.
2. Path-aware commands MUST also accept positional `<path>`.
3. If both positional path and `--path` are provided and values differ, the command MUST fail with `ValidationError`.

Resource source flags:
1. `resource get` and `resource list` MUST support `--source <remote-server|repository>`.
2. `resource delete` MUST support `--source <remote-server|repository|both>`.
3. `resource get|list|delete` MUST default `--source` to `remote-server`.
4. Legacy source booleans (`--repository`, `--remote-server`, and `--both` for delete) MAY remain as hidden compatibility aliases during migrations.

Core resource commands:
1. `get`.
2. `save`.
3. `apply`.
4. `create`.
5. `update`.
6. `delete`.
7. `diff`.
8. `list`.
9. `explain`.
10. `template`.
11. `edit`.
12. `copy`.
13. `request`.

Selected command names:
1. `config edit`.
2. `config print-template`.
3. `config add`.
4. `config use`.
5. `config show`.
6. `config current`.
7. `config resolve`.
8. `metadata resolve`.
9. `metadata render`.
10. `repo status`.
11. `repo clean`.
12. `repo commit`.
13. `repo history`.
14. `repo tree`.
15. `resource request`.
14. `secret mask`.
15. `secret resolve`.
16. `secret normalize`.
17. `secret detect`.
18. `completion`.
19. `version`.
20. `resource-server get base-url`.
19. `resource-server get token-url`.
20. `resource-server get access-token`.
21. `resource-server check`.

HTTP request command methods:
1. Canonical path is `resource request <method>`.
2. Methods: `get|head|options|post|put|patch|delete|trace|connect`.

Interactive config commands:
1. `config add` SHOULD support terminal prompts when no file/stdin input is provided.
2. `config add` SHOULD accept optional context name from positional `[new-context-name]` or global `--context` and skip name prompting when provided.
3. `config use` SHOULD support context selection when no name argument is provided.
4. `config show` SHOULD support context selection when `--context` is omitted.
5. `config rename` SHOULD support context selection and target-name prompt when arguments are omitted.
6. `config delete` SHOULD support context selection and explicit confirmation when no name argument is provided.
7. Interactive `config add` SHOULD treat `resource-server` as required and SHOULD still surface optional sections with explicit skip choices (for example `secret-store` and `preferences`).
8. For interactive `config add`, repository `resource-format` SHOULD be optional with an explicit remote-default selection.
9. `config edit` SHOULD open the context catalog in an editor, validate the edited YAML on save/exit, and persist only validated changes.
10. `config edit <name>` SHOULD present only the selected context for editing and merge the validated result back into the full catalog.

## CLI Input Grammar
1. Resource targets MUST be logical absolute paths.
2. Metadata targets accept collection and resource scopes with positional path and `--path`.
3. Metadata selector paths using intermediary namespace segments (for example `/admin/realms/_/clients/`) MUST be accepted by `metadata get|infer|render`.
4. `metadata render` MUST accept optional operation input; when omitted, it MUST default to `list` for collection/selector targets and `get` for resource targets.
5. When `metadata render` defaults to `get` and the resolved `get` operation path is missing, it MUST retry with `list` before returning a validation error.
6. `metadata infer` MUST use OpenAPI path hints when available and MUST still return deterministic fallback inference when OpenAPI is unavailable.
7. `metadata infer` output MUST omit inferred directives that are equal to deterministic fallback defaults for the requested target.
8. `metadata infer --apply` MUST persist the same compacted metadata payload shown in command output and MUST NOT persist inferred directives equal to defaults; when JSON is selected, both infer output and persisted metadata JSON MUST end with one trailing newline.
9. `metadata get` MUST return resolved repository metadata overrides merged with default metadata fields by default; `metadata get --overrides-only` MUST return only the resolved/inferred override object without merged defaults.
10. When metadata overrides are missing, `metadata get` MUST return inferred metadata (compact in `--overrides-only` mode, default-merged in standard mode) when the target endpoint exists in OpenAPI or is reachable from the resource server; otherwise it MUST keep the `NotFoundError`.
11. Mutations from stdin MUST validate payload format before side effects.
12. Option conflicts MUST produce usage errors.
13. Shell completion output SHOULD avoid duplicate flag suggestions that differ only by `=` suffix (for example `--output` and `--output=`).
14. `resource get` MUST support `--source <remote-server|repository>`.
15. `resource get` MUST default to `--source remote-server`, and remote reads MUST attempt the literal path first then list/filter by metadata-derived identity when the literal read returns `NotFound`.
16. `resource get` MUST redact values for metadata-declared `resourceInfo.secretInAttributes` using `{{secret .}}` placeholders for both `--source repository` and `--source remote-server` output by default.
17. `resource get --show-secrets` MUST disable metadata-driven output redaction and print plaintext values.
18. `resource get --show-metadata` MUST include the rendered metadata snapshot (default metadata merged with overrides) alongside the payload, presenting resolved `operationInfo.*` directives under the metadata section.
19. `resource list` MUST support `--source <remote-server|repository>`.
20. `resource list` MUST default to `--source remote-server`.
21. `resource list` MUST support `--recursive` and default to non-recursive direct-child listing.
22. `resource delete` MUST support `--source <remote-server|repository|both>` and default to `--source remote-server`.
23. `resource delete` MUST support `--recursive` and default to non-recursive collection deletes.
24. `resource apply` MUST treat collection paths as batch targets resolved from local repository resources and default to non-recursive direct-child execution when payload input is absent.
25. `resource apply --recursive` MUST include descendant resources under the target path when payload input is absent.
26. `resource apply` MUST accept explicit payload input (`--payload` file path, `-`/stdin, inline JSON/YAML object text, or dotted assignments) for a single target path and use explicit input instead of loading repository payloads; explicit-input apply MUST perform update when the remote resource exists and create when it does not.
27. `resource apply` MUST reject `--recursive` when explicit payload input is provided.
28. `resource create` MUST accept explicit payload input (`--payload` file path, `-`/stdin, inline JSON/YAML object text, or dotted assignments) for a single remote mutation, and when payload input is absent it MUST load local repository payloads for resources under the target path and execute create for each resolved target.
29. `resource update` MUST accept explicit payload input (`--payload` file path, `-`/stdin, inline JSON/YAML object text, or dotted assignments) for a single remote mutation, and when payload input is absent it MUST load local repository payloads for resources under the target path and execute remote updates for each matching resource.
30. `resource update` without `--recursive` MUST mutate only direct-child resources for collection paths when payload input is absent.
31. `resource update --recursive` MUST include descendant resources under the target path when payload input is absent.
32. `resource apply|create|update` explicit-input mode MUST accept a collection target path when metadata exposes a wildcard child under that path and the payload contains metadata-derived alias/id values; the CLI MUST infer one concrete child logical path from the payload identity before remote mutation, and explicit resource-path inputs MUST preserve identity-match validation behavior.
32. `resource apply` explicit-input mode (`--payload` file path, `-`/stdin, inline JSON/YAML object text, or dotted assignments) MUST use upsert semantics for the target path: create when remote resource lookup returns `NotFound`, otherwise update.
33. `resource save` without payload input (`--payload` omitted) MUST read the requested path from the remote server and persist the value into the repository, using the same literal-then-list/filter metadata-aware fallback as `resource get`.
34. `resource save` MUST support optional explicit payload input from `--payload` as file path, `-` (stdin), inline JSON/YAML object text, or dotted assignments.
32. `resource save` MUST support mutually exclusive `--as-items` and `--as-one-resource` flags.
33. `resource save` MUST default to `--as-items` behavior when input payload is a list (`[]` or object with `items` array).
34. `resource save` MUST automatically store and mask detected plaintext secret candidates declared by metadata `resourceInfo.secretInAttributes` before repository persistence; non-metadata-declared candidates MUST fail with `ValidationError` unless `--ignore` or `--handle-secrets` is set; if the logical path already exists in the repository, overriding the persisted resource MUST additionally require `--overwrite`.
35. `resource save --handle-secrets` MUST accept an optional comma-separated attribute list; when no list is provided, all detected plaintext secret candidates MUST be handled.
36. `resource save --handle-secrets` MUST detect plaintext secret attributes, store handled values in the configured secret store using path-scoped keys, replace handled payload values with `{{secret .}}` placeholders, and merge handled attributes into metadata `resourceInfo.secretInAttributes` for the saved logical path.
37. Resource payload placeholder resolution for remote workflows MUST resolve `{{secret .}}` as `<logical-path>:<attribute-path>`, resolve `{{secret <custom-key>}}` as `<logical-path>:<custom-key>`, and remain compatible with legacy absolute key placeholders.
38. When `resource save --handle-secrets` handles only a subset of detected candidates, the command MUST fail with the same plaintext-secret warning using only unhandled candidates that are not metadata-declared, unless `--ignore` is set.
39. For collection list saves (`--as-items` default), plaintext-secret candidate detection MUST be computed once per save from the collection payload set and then applied consistently across all list items.
40. `resource edit` MUST open the local repository payload in an editor, validate the edited payload before persistence, and persist the edited payload only when decoding and save validation succeed.
41. `resource edit` on git repository contexts MUST commit repository changes and MAY auto-sync when the git context enables repository auto-sync.
42. `resource copy` MUST support positional `[path] [target-path]` and flag-driven `--path` plus `--target-path` inputs, and mismatched positional/flag target values MUST fail with `ValidationError`.
43. `resource copy --overrides` MUST accept dotted assignments (`a=b,c=d,e.f.g=h`) and apply them to object payloads before save validation.
44. `resource copy` MUST read the source path from the local repository first and, when that lookup returns `NotFoundError`, retry the source read from the remote server before applying overrides and save validation.
44. `secret detect` MUST support optional `--fix` to persist detected attributes into metadata `resourceInfo.secretInAttributes`.
45. `secret detect` without input payload (`--payload <path|->` or stdin) MUST scan local repository resources recursively under positional `<path>`/`--path`, defaulting to `/` when path is omitted.
46. `secret detect --fix` in input-payload mode MUST require a target path from positional `<path>` or `--path`.
47. `secret detect --fix` in repository-scan mode MUST merge detected attributes into metadata `resourceInfo.secretInAttributes` for each detected resource path in scope.
48. `secret detect --secret-attribute <attr>` MUST apply only that detected attribute and MUST fail with `ValidationError` when the requested attribute is not detected in payload or repository scope.
49. `secret get` MUST accept `secret get <path>`, `secret get <path> <key>`, `secret get --path <path>`, `secret get --path <path> --key <key>`, and `secret get <path>:<key>` in addition to direct key mode (`secret get <key>`).
50. `secret get <path>` and `secret get --path <path>` MUST list all path-scoped secrets whose keys start with `<path>:` in deterministic key order.
51. `secret get` with path+key input (`<path> <key>`, `--path`+`--key`, or `<path>:<key>`) MUST resolve the canonical secret key as `<path>:<key>`.
52. `secret get --key` MUST require `--path`.
49. Interactive config flows MUST fail fast with `ValidationError` when invoked without required arguments in non-interactive environments.
50. `config show` MUST use `--context` when provided and otherwise require interactive context selection.
51. `config add` MUST default `--format` to `yaml` while continuing to accept explicit `json`.
52. `config add` MUST accept optional context name from positional `[new-context-name]` or global `--context`.
53. `config add` MUST fail with `ValidationError` when positional `[new-context-name]` and global `--context` are both provided with different values.
54. `resource request <method>` MUST accept endpoint path from positional `<path>` and `--path`, and mismatched values MUST fail with `ValidationError`; `resource request get` MUST attempt metadata-aware remote read fallback when the literal request returns `NotFound`.
55. `resource request <method>` MUST accept optional request payload from `--payload <path|->` or stdin, decoding according to `--format` (`json|yaml`) when payload input is provided.
56. `resource request post` and `resource request put` MUST also support optional inline `--payload` input, decoded according to `--format`, and the inline `--payload` MUST be mutually exclusive with the `--payload <path|->`/stdin option.
56. `config add` MUST accept input from `--file <path|->` or stdin.
57. `config add` MUST accept either one `context` object or one full `contexts.yaml` catalog object.
58. When `config add` receives a catalog input and `--context-name` is omitted, it MUST import all catalog contexts.
59. When `config add` receives a catalog input and `--context-name` is set, it MUST import only the matching catalog context name.
60. When `config add` receives a single context object and `--context-name` is set, the imported context name MUST be overridden by `--context-name`.
61. `config add --set-current` MUST set current context to the imported context when exactly one context is imported.
62. `config add --set-current` with multiple imported contexts MUST require catalog `current-ctx` or fail with `ValidationError`.
63. `config add` SHOULD default `--format` to `yaml` while continuing to accept explicit `json`.
64. `config update` and `config validate` SHOULD default `--format` to `yaml` while continuing to accept explicit `json`.
65. Help and completion-script invocations MUST bypass context-dependent startup validation so command usage remains available when no current context is configured.
66. Command-group invocations without subcommands MUST bypass context-dependent startup validation so usage/help output remains available when no current context is configured.
67. `resource request delete` MUST require `--confirm-delete` and fail with `ValidationError` when confirmation is not explicit.
68. Repository-backed single-resource reads (`resource get --source repository`, `resource apply`, `resource update`, `resource diff`, `resource explain`) MUST attempt literal repository lookup first and, on `NotFound`, perform a bounded collection fallback that matches by metadata `idFromAttribute`.
69. `resource apply|create|update` collection-target resolution MUST attempt a non-recursive collection list first and, when no entries match a deep path target, attempt single-resource fallback lookup before returning `NotFound`.
70. `resource delete --source remote-server` MUST resolve collection targets from local repository resources (direct-child by default, descendants with `--recursive`) and, when no local targets match, attempt literal delete with metadata-aware remote identity fallback on `NotFound`.
71. `resource request delete` MUST resolve collection targets from local repository resources (direct-child by default, descendants with `--recursive`) and issue one delete request per resolved target; when no local targets match it MUST issue a single delete request for the requested path.
72. `resource save` MUST accept `_` as a wildcard path segment when no payload input is provided and MUST expand each wildcard level through remote direct-child list lookups before saving resolved targets.
73. `resource save` with wildcard path segments and payload input (`--payload <path|->` or stdin) MUST fail with `ValidationError`.
74. `resource save` wildcard expansions for resource targets MUST skip unresolved concrete `NotFound` reads and MUST return `NotFoundError` when no concrete targets resolve successfully.
75. `resource diff` MUST resolve collection targets from local repository resources (direct-child by default), execute compare for each resolved resource, and when no collection targets match a deep path it MUST attempt single-resource fallback lookup before returning `NotFound`.
76. Interactive `config add` MUST support full context-schema authoring: prompt required fields for repository and resource-server providers, offer skip paths for optional sections, and enforce one-of prompt branching (for example oauth2 vs basic-auth) by collecting only the selected option's fields.
77. `config print-template` MUST output a commented YAML context catalog template that includes all supported configuration branches and explicitly marks mutually-exclusive blocks.
78. `repo push` MUST fail with `ValidationError` when the active repository type is `filesystem`, and it MUST fail with `ValidationError` when active repository type is `git` without `repository.git.remote` configuration.
79. `repo clean` MUST discard uncommitted tracked and untracked changes for git repositories and MUST succeed as a no-op for filesystem repositories.
80. `repo commit` MUST accept `--message`, `-m`, fail with `ValidationError` when the active repository type is `filesystem`, and create at most one local git commit from current worktree changes.
81. `repo commit` on a clean git worktree MUST succeed as a no-op and report that no commit was created.
82. `repo status --verbose` (global `--verbose`) MUST include deterministic local worktree change details for git repositories.
79. Context-catalog mutations (`config add|edit|update|validate`) MUST fail validation when `resource-server` is omitted.
80. Interactive `config add` MUST offer a `resource-format` remote-default option that omits explicit `repository.resource-format`.
81. `repo history` MUST return a deterministic not-supported text message for filesystem repositories and MUST expose filtered local git history for git repositories.
82. `repo tree` MUST accept no positional arguments and MUST print a deterministic directory-only tree view of the local repository, excluding files, hidden control directories (for example `.git`), and reserved metadata namespace directories named `_`; directory names with spaces MUST be preserved verbatim.
82. `resource create|apply` explicit-input payload mode MUST fail with `ValidationError` when metadata identity attributes (`aliasFromAttribute` or `idFromAttribute`) present in the payload do not match the target path segment.
83. `resource save` on a git repository context MUST create a local commit after repository mutation and MUST accept `--message` (append to default message) and `--message-override` (replace default message); the flags MUST be mutually exclusive.
84. `resource delete` when repository deletion is selected (`--source repository|both` or legacy `--repository|--both`) on a git repository context MUST create a local commit after repository mutation and MUST accept the same commit-message flags with the same mutual-exclusion rule.
85. Auto-commit-enabled repository mutation commands (`resource save|delete|edit`) MUST require a clean git worktree before mutation to avoid committing unrelated changes.
86. Commands that open editors (`config edit`, `resource edit`) MUST support `--editor <command>` to override the catalog `default-editor` and the built-in `vi` fallback.
87. Git-backed repository command flows and git-backed repository mutation post-actions (for example `repo status|clean|history|check|refresh|reset|push` and resource auto-commit/status checks) MUST auto-initialize the local git repository when `.git/` is missing before continuing operation-specific behavior.
88. `resource get` with an explicit trailing slash collection marker and remote source (`--source remote-server` or default) MUST execute remote list resolution for the normalized collection path first; when that list attempt fails with list-response shape validation (`list response ...` or `list payload ...`), the command MUST retry a single-resource remote read for the same normalized path.
89. Path completion candidates containing spaces in non-terminal segments (for example `/admin/realms/publico-br/user-registry/AD PRD`) MUST be preserved as one completion token in generated shell completion scripts.
90. `resource-server get base-url` MUST print the active context `resource-server.http.base-url` and fail with `ValidationError` when `resource-server.http` is not configured.
91. `resource-server get token-url` MUST print the active context `resource-server.http.auth.oauth2.token-url` and fail with `ValidationError` when OAuth2 auth is not configured.
92. `resource-server get access-token` MUST fetch and print the OAuth2 access token from `resource-server.http.auth.oauth2`; when OAuth2 auth is not configured, it MUST fail with `ValidationError`.
93. `resource-server check` MUST probe resource-server connectivity using a non-recursive remote root list request and treat probe results in categories `NotFoundError`, `ValidationError`, and `ConflictError` as reachable-success outcomes while surfacing other errors.
94. Commands with plain-text-only output (`secret get`, `repo tree`, `resource-server get *`, `resource-server check`, shell `completion` subcommands, and `config print-template`) MUST reject `--output json|yaml` with `ValidationError` instead of silently ignoring the requested format.
95. `config show` MUST print YAML by default and MUST reject `--output json`; it MAY accept `--output text` or `--output yaml`.
96. `repo status --verbose` MAY include structured `worktree` entries when `--output json|yaml` is selected.
97. `repo commit` MUST support `--output text|json|yaml`; `json|yaml` output MUST expose a stable `committed` success indicator, and text-mode success SHOULD use the standard CLI execution-status footer.

## Output Contract
1. Success output MAY be human-readable by default.
2. Structured output mode MUST define stable keys for automation.
3. Error output MUST include category and actionable next step where possible.
4. Diff output MUST present deterministic ordering.
5. When `--output` is `auto` (default), resource-oriented output MUST follow the active context `repository.resource-format` (`json` or `yaml`).
6. `repo status` with `--output auto` MUST render deterministic text summary by default.
7. `repo status --verbose` text output MUST append deterministic git-style short worktree detail lines for git repositories and MUST print `worktree=clean` when no local changes exist.
8. `repo commit` text output MUST deterministically report whether a commit was created (including the clean-worktree no-op case).
9. `repo tree` text output MUST render a deterministic tree-style directory listing using repository-relative directory names only (no files), preserving spaces within directory segments.
9. `resource list --output text` MUST render one line per item in the form `<alias> (<id>)`, preferring metadata-derived identity (`aliasFromAttribute`/`idFromAttribute`) and falling back to resolved item identity fields when already present.
7. `config show` MUST print the full selected context configuration as YAML to stdout.
8. Command help output MUST present `--help` in the `Global Flags` section.
9. HTTP transport debug output MUST include TLS/mTLS configuration context (`tls_enabled`, `mtls_enabled`, and configured TLS file paths) without logging secret values.
10. Help output SHOULD avoid repeated blank lines between sections.
11. `resource diff --output text` MUST render one line per diff entry using relative dot-path notation from the requested target path and JSON-encoded values in the form `<dot-path> [Local=<json>] => [Remote=<json>]`.
12. Structured `resource diff` and `resource explain` output MUST encode `resource.DiffEntry.ResourcePath` as the logical resource path and `resource.DiffEntry.Path` as an RFC 6901 JSON Pointer relative to that resource payload; root payload replacements MUST use `Path=""`.
13. Unless `--no-status` is set, resource-mutation commands (`resource save|apply|create|update|delete`) and state-changing HTTP request commands (`resource request post|put|patch|delete|connect`) MUST print a terminal status line as the final output line to stderr using `[OK] <description>.` on success and `[ERROR] <description>.` on failure.
14. Interactive terminal status output SHOULD render `[OK]` in bold green and `[ERROR]` in bold red.
15. Commands returning nil payload output MUST emit no payload body (no `null`/`<nil>` placeholder output).
16. Metadata command structured output (`metadata get|resolve|infer`) MUST omit nil directive fields instead of emitting `null` entries.
17. Metadata JSON command output and persisted JSON metadata produced by `metadata infer --apply` MUST end with one trailing newline.
18. State-changing commands (`resource save|apply|create|update|delete` and `resource request post|put|patch|delete|connect`) MUST suppress complementary payload output by default and print only the status footer.
19. `--verbose` MUST re-enable complementary payload output for commands that suppress it by default.
20. `config check` text output MUST report component rows using `context`, `repository`, `metadata`, `resource-server`, and `secret-store` labels.
21. `repo status` text output MUST be repository-type aware: `filesystem` contexts MUST report local-only sync as `sync=not_applicable`, and `git` contexts MUST report git sync state with explicit `remote=not_configured` marker when remote configuration is absent.
22. `secret get` output MUST always be plain text: single-secret reads print only the secret value line, and path reads print one `<key>=<value>` line per matched secret without JSON quoting.
23. `resource-server get base-url|token-url|access-token` output MUST be plain text and print only the requested value followed by one trailing newline.
24. `resource-server check` text output MUST print a concise probe result line and MAY include one additional line with the probe error detail when connectivity is confirmed through a warning-category response.
25. The CLI MUST support `--no-color` to disable ANSI color output for status labels, and `NO_COLOR` environment variable MUST also disable ANSI color output.
26. CLI process exit codes SHOULD map typed error categories deterministically (for example validation, not-found, auth, conflict, transport) instead of collapsing all errors to one exit code.

## Failure Modes
1. Missing required path argument.
2. Invalid payload format.
3. Unsupported command/flag combination.
4. Command requires configured manager not present in active context.
5. `resource get|list|delete` receives invalid `--source` values.
6. `resource get|list|delete` receives `--source` combined with legacy source aliases during compatibility periods.
7. `resource get|list|delete` receives conflicting legacy source flags (`--repository`, `--remote-server`, `--both`).
8. `resource save` receives both `--as-items` and `--as-one-resource`.
9. `resource save --as-items` receives non-list input.
10. `resource save` detects non-metadata-declared potential plaintext secret values and neither `--ignore` nor `--handle-secrets` is set, detects metadata-declared plaintext candidates without a configured secret provider, or attempts to overwrite an existing repository resource without `--overwrite`.
11. `resource save --handle-secrets=<attr-list>` includes one or more attributes that are not detected in the payload.
12. `resource create` is invoked without payload input and no matching local resources exist under the target path.
13. `resource apply`, `resource create`, or `resource update` targets a collection path with no local resources.
14. `secret detect --fix` is provided with payload input but without path input.
15. `secret detect --secret-attribute` value is not detected in payload or repository scope.
16. `config add --context-name` does not match any catalog context.
17. `resource request post|put` receives inline `--payload` together with the `--payload <path|->` or stdin option.
19. `config add --set-current` with multiple imported contexts and missing catalog `current-ctx`.
20. `resource request delete` is invoked without `--confirm-delete`.
21. Metadata-aware identity fallback yields multiple candidates for the same requested path and returns `ConflictError`.
22. `resource save` wildcard path is combined with payload input.
23. `resource diff` targets a collection path with no local resources.
24. `config add` receives both positional context name and `--context` with different values.
25. `config print-template` receives positional arguments.
26. `repo push` is invoked for a `filesystem` repository context.
27. Context-catalog mutation input omits required `resource-server`.
28. `secret get --key <key>` is invoked without `--path`.
29. `secret get <path>:` uses an empty key segment.
30. `resource-server get token-url` or `resource-server get access-token` is invoked when the active context resource-server auth mode is not OAuth2.
31. `resource save|delete` receives both `--message` and `--message-override`.
32. `resource save|delete` auto-commit is attempted while the git worktree already has unrelated uncommitted changes.

## Edge Cases
1. `resource save` encounters plaintext secret candidates selected for handling (automatic metadata-declared handling or `--handle-secrets`) but no secret manager is configured.
2. `resource save --handle-secrets` handles only a subset and fails with warning for the remaining non-metadata-declared plaintext candidates unless `--ignore` is set.
3. `delete` invoked on collection without recursive force confirmation.
4. `metadata infer` called with missing OpenAPI source.
5. Completion for alias path when remote ID differs.
6. Interactive config command invoked from non-TTY input/output.
7. `resource save --help` invoked when no current context exists.
8. `completion` or shell completion engine invocation (`__complete`, `__completeNoDesc`) when no current context exists.
9. `resource save` payload mixes placeholders and plaintext values for secret-like fields.
10. Root command completion includes command help entry as `help` and excludes internal aliases.
11. `secret detect --fix` in repository-scan mode updates metadata for paths that currently have no metadata files.
12. `resource` invoked without a subcommand when no current context exists.
13. `resource apply`, `resource create`, or `resource update` is invoked on a collection that has only nested descendants and omits `--recursive`.
14. `resource save` list payload item is missing metadata-defined alias/id attributes; command falls back to common identity attributes (`clientId`, `id`, `name`, `alias`) before failing.
15. Repository identity fallback receives a path segment that matches multiple resources by metadata `idFromAttribute` and fails with `ConflictError`.
16. `resource save /admin/realms/_/clients/test` expands wildcard realms, skips `NotFound` resources for missing `test` clients, and fails only when no realm contains a match.
17. `resource diff` collection targets include only direct-child local resources and exclude nested descendants.
18. Completion for a templated OpenAPI path segment with a partial value (for example `/admin/rea`) returns canonical concrete collection candidates (for example `/admin/realms/`) when local/remote/OpenAPI context provides them and suppresses template placeholder segments (`{...}`) from completion output.
19. `version` and context-catalog management commands (for example `config list`) succeed when no current context is set, while runtime commands continue to fail fast when active context resolution is required.
20. `secret get /customers/acme` prints multiple lines in deterministic order as `<key>=<value>` and preserves quote characters only when they exist in secret values.
21. `config add` with resource-server auth set to `oauth2` prompts only oauth2 fields and does not prompt `basic-auth`, `bearer-token`, or `custom-header` fields.
22. `config print-template` works without a configured current context and still renders the full template.
23. `repo status` in a `filesystem` context prints `sync=not_applicable` instead of git `ahead/behind` counters.
24. `repo clean` in a `filesystem` context succeeds without repository mutations and leaves output empty.
24. Interactive `config add` with `resource-format=remote-default` stores no explicit `repository.resource-format` value.
25. `resource list --output text` falls back to logical-path alias formatting when metadata identity attributes are absent from an item payload.
25. `resource get /admin/realms/master/` first attempts remote list for `/admin/realms/master` and then falls back to one remote single-resource read when the list response shape is invalid.
26. `resource-server get token-url` or `resource-server get access-token` is invoked for a context configured with `basic-auth`, `bearer-token`, or `custom-header` and fails with `ValidationError`.
27. `resource-server check` reaches the server but the root-list probe returns `NotFoundError`, `ValidationError`, or `ConflictError`, and the command still reports connectivity success with the probe detail.
28. Path completion for `/admin/realms/_/clients/` preserves `_` selector segments as canonical logical metadata-path suggestions instead of replacing `_` with placeholder text.

## Examples
1. `declarest resource apply /customers/acme` applies desired state for one resource.
2. `declarest resource apply --path /customers/acme` applies desired state for one resource using flag input.
3. `declarest resource apply /customers` applies all direct-child local resources in `/customers`.
4. `declarest resource apply /customers --recursive` applies direct and nested resources under `/customers`.
5. `declarest resource apply /customers/acme --payload payload.json` applies explicit payload and overrides repository-sourced input for that target path.
6. `cat payload.json | declarest resource apply /customers/acme --payload -` applies explicit payload from stdin and overrides repository-sourced input for that target path.
7. `declarest resource create /customers` creates all direct-child resources in `/customers` using repository payloads.
8. `declarest resource create /customers/acme --payload payload.json` creates one remote resource from explicit payload input (overrides repository-sourced input for that target path).
9. `cat payload.json | declarest resource create /customers/acme --payload -` creates one remote resource from stdin payload input (overrides repository-sourced input for that target path).
10. `declarest resource update /customers` updates only direct-child resources in `/customers` using repository payloads and skips nested descendants.
11. `declarest resource update /customers --recursive` updates direct and nested resources under `/customers` using repository payloads.
12. `declarest resource update /customers/acme --payload payload.json` updates one remote resource from explicit payload input (overrides repository-sourced input for that target path).
13. `cat payload.json | declarest resource update /customers/acme --payload -` updates one remote resource from stdin payload input (overrides repository-sourced input for that target path).
14. `declarest resource get /customers/acme` reads remote state by default.
15. `declarest resource get /customers/acme --source repository` reads local repository state.
16. `declarest resource get /customers --source repository` lists repository resources under `/customers`, mirroring `declarest resource list /customers --source repository`.
17. `declarest resource list /customers` lists remote resources by default.
18. `declarest resource list /customers --source repository` lists repository resources.
19. `declarest resource list /customers --output text` prints one `<alias> (<id>)` line per listed item using metadata-derived identity when available.
19. `declarest resource delete /customers/acme --confirm-delete` deletes from the remote server by default.
20. `declarest resource delete /customers/acme --confirm-delete --source both` deletes from both remote server and repository.
21. `declarest resource save /customers/acme` fetches remote state and saves it into repository for `/customers/acme`.
22. `declarest resource save /customers < list.json` stores each list item as its own resource when `list.json` is a list payload.
23. `declarest resource save /customers --as-one-resource < list.json` stores the list payload in one resource file.
24. `declarest resource save /customers/acme < payload.json` fails with `ValidationError` when plaintext secret candidates are detected.
25. `declarest resource save /customers/acme --ignore < payload.json` bypasses plaintext-secret save guard.
26. `declarest resource save /customers/acme < payload.json` with metadata `resourceInfo.secretInAttributes: [credentials.authValue]` stores and masks `credentials.authValue` automatically before repository persistence.
27. `declarest resource save /customers/acme --handle-secrets < payload.json` stores all detected secrets, masks payload values with placeholders, and updates metadata `resourceInfo.secretInAttributes`.
28. `declarest resource save /customers/acme --handle-secrets=password < payload.json` handles only `password`; if other candidates remain, command fails with warning listing only the unhandled candidates unless `--ignore` is set.
29. `declarest secret detect /customers/acme --fix < payload.json` detects secret attributes and writes them to metadata `resourceInfo.secretInAttributes` for `/customers/acme`.
30. `declarest secret detect /customers/acme --fix --secret-attribute password < payload.json` writes only `password` from detected candidates.
27. `declarest secret get /customers/acme` prints all path-scoped secrets for `/customers/acme` as plain text lines.
28. `declarest secret get /customers/acme apiToken` prints only the secret value for `/customers/acme:apiToken`.
29. `declarest secret get --path /customers/acme --key apiToken` prints only the secret value for `/customers/acme:apiToken`.
30. `declarest secret get /customers/acme:apiToken` prints only the secret value for `/customers/acme:apiToken`.
31. `declarest resource save /admin/realms/master/clients/` saves remote list items using metadata identity attributes and falls back to common attributes like `id` when metadata attributes are absent in payload entries.
32. `declarest metadata infer --path /customers --apply --recursive` writes inferred metadata recursively.
33. `declarest metadata render /customers/acme get` renders metadata operation spec.
34. `declarest metadata infer /admin/realms/_/clients/` infers selector-path metadata using OpenAPI hints when available.
35. `declarest metadata render /admin/realms/_/clients/` defaults to rendering the `list` operation for the selector collection path.
36. `declarest repo push --force-push` executes force push with explicit safety acknowledgment.
37. `declarest repo status` reports local/remote sync status without mutating repository state.
38. `declarest repo status --verbose` prints sync summary plus git-style local worktree details (for example modified or untracked files) in git contexts.
39. `declarest repo commit -m "manual changes"` commits manual local repository changes in git contexts and reports whether a commit was created.
40. `declarest repo clean` discards uncommitted tracked and untracked changes in a git repository worktree.
41. `declarest repo tree` prints a tree-style directory view of the local repository and preserves spaces in directory names (for example `AD PRD`) while omitting files.
38. `declarest completion bash` generates Bash completion output.
39. `declarest version -o json` prints machine-readable version information.
40. `declarest config use` opens interactive context selection when run in a terminal.
41. `declarest config show --context dev` prints the selected context configuration as YAML.
42. `declarest resource request get /health` executes a direct resource-server GET request.
43. `declarest resource request post /customers --payload payload.json` executes a direct resource-server POST request with JSON body.
44. `declarest resource request post /customers --payload '{"id":"acme"}'` executes a direct resource-server POST request with inline JSON payload.
45. `echo '{"id":"acme"}' | declarest resource request put /customers/acme` executes a direct resource-server PUT request from stdin.
46. `declarest resource request delete /customers/a --path /customers/b` fails with `ValidationError` due to path mismatch.
47. `declarest config add` opens interactive prompts to build one context configuration when `--file`/stdin input is not provided.
48. `declarest config add --file contexts.yaml --format yaml` imports all contexts defined in a catalog file.
49. `declarest config add --file contexts.yaml --format yaml --context-name prod --set-current` imports only `prod` and sets it as current.
50. `declarest config add --file contexts.yaml --format yaml --set-current` fails when multiple contexts are imported and the catalog omits `current-ctx`.
51. `declarest resource save --help` prints help text even when no current context is configured.
52. `declarest secret detect` scans the whole local repository for secret candidates when no payload input is provided.
53. `declarest secret detect /customers --fix` scans local resources under `/customers` and updates metadata `resourceInfo.secretInAttributes` for detected resource paths.
54. `declarest completion bash` prints completion script even when no current context is configured.
55. `declarest` shell tab completion at root suggests `help` and does not suggest internal helper names.
56. `declarest resource` prints resource command help even when no current context is configured.
57. `declarest resource request delete /customers/acme` fails with `ValidationError` because `--confirm-delete` is required.
58. `declarest resource request delete /customers/acme --confirm-delete` executes a direct resource-server DELETE request.
59. `declarest resource request delete /customers --confirm-delete --recursive` issues delete requests for all repository resources under `/customers`.
60. `declarest resource apply /admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c` applies the local client resource whose metadata `idFromAttribute` matches the provided path segment when no literal repository resource exists.
61. `declarest resource delete /admin/realms/master/clients/account --confirm-delete --source remote-server` retries deletion using metadata-resolved remote ID when the literal delete path is not found.
62. `declarest resource save /admin/realms/_/clients/` expands wildcard realms and saves clients from all matched realms.
63. `declarest resource save /admin/realms/_/clients/test` expands wildcard realms and saves each matched `test` client resource path.
64. `declarest resource diff /customers` compares all direct-child repository resources in `/customers` and returns a single deterministic diff list.
65. `declarest resource diff /admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c` falls back to single-resource lookup when collection resolution for that deep path has no direct matches.
66. `declarest resource diff /admin/realms/payments --output text` prints lines like `.displayName [Local="Payments Realm"] => [Remote="Payments Realm 2"]`.
67. `declarest resource save /customers/acme -f payload.json -i json` terminates with `[OK] command executed successfully.`.
68. `declarest resource save /customers/acme -f payload.json -i json --no-status` suppresses the final status line.
69. `declarest resource request get /health` terminates with `[OK] command executed successfully.`.
70. `declarest resource create /customers/acme --payload payload.json` prints no payload output by default and only the final status footer.
71. `cat payload.json | declarest resource create /customers/acme --payload - --verbose` prints the created target payload output plus the final status footer.
72. `declarest resource request delete /customers --confirm-delete --recursive` prints no response bodies by default and only the final status footer.
73. `declarest resource request delete /customers --confirm-delete --recursive --verbose` prints response bodies for each resolved delete target plus the final status footer.
74. `declarest resource get /admin/realms/m<TAB>` completes to concrete candidates such as `/admin/realms/master` by combining OpenAPI templates with local/remote collection item lookups.
75. `declarest config add dev` skips context-name prompt and starts interactive prompts at repository settings.
76. `declarest config add --context dev` skips context-name prompt and starts interactive prompts at repository settings.
77. `declarest config add full` can populate resource-server, secret-store, TLS, and preference fields interactively while allowing optional sections to be skipped.
78. `declarest config print-template` prints a full commented `contexts.yaml` template including mutually-exclusive option guidance.
79. `declarest repo push` fails with `ValidationError` when the active context repository type is `filesystem`.
80. `declarest repo status` in a filesystem context prints `type=filesystem sync=not_applicable hasUncommitted=<bool>`.
81. `declarest config add` interactive flow always prompts `resource-server` fields and allows `resource-format` to remain unset via remote-default selection.
82. `declarest config edit prod --editor "vi"` opens a temporary YAML document for only `prod`, validates it on save/exit, and replaces the stored `prod` context only when validation succeeds.
83. `declarest resource edit /customers/acme --editor "vi"` opens the local repository payload, validates the edited content, and commits changes when the repository backend is git.
84. `declarest resource copy /customers/acme /customers/acme-copy --overrides name=acme-copy,spec.tier=gold` copies one repository resource and applies dotted overrides before saving.
85. `declarest resource copy /admin/realms/test /admin/realms/test2 --overrides realm=test2` falls back to the remote read when the source realm is not yet saved locally and updates the identity attribute to match the target path.
85. `declarest resource save /customers/acme --payload 'id=acme,name=Acme' --overwrite --message ticket-123` saves a resource and appends `ticket-123` to the default git commit message when the active repository is git.
86. `declarest resource delete /customers/acme --confirm-delete --repository --message-override 'cleanup customer'` deletes from the repository and commits with the overridden git commit message in a git context.
87. `declarest repo history --oneline --max-count 5 --author alice --grep fix --path customers` prints filtered local git commit history.
88. `declarest repo history` in a filesystem context prints a deterministic not-supported message.
82. `declarest resource get /adm<TAB>` completes to `/admin/`; when remote completion lookups fail, completion falls back to repository candidates.
83. `declarest resource get /admin/realms/master/clients/<TAB>` completes using alias values from metadata `aliasFromAttribute` (for example `account`) instead of ID-only segments.
84. `declarest resource get /admin/realms/publico-br/user-registry/AD/mappers/` executes remote collection list resolution for `/admin/realms/publico-br/user-registry/AD/mappers`.
85. `declarest resource get /admin/realms/publico-br/user-registry/A<TAB>` can complete to `/admin/realms/publico-br/user-registry/AD PRD` as one candidate path segment.
86. `declarest resource get /admin/realms/master/` retries a single-resource remote read for `/admin/realms/master` when collection list decoding fails with `list response ...` or `list payload ...` validation.
87. `declarest resource-server get base-url` prints the active context resource-server HTTP base URL.
88. `declarest resource-server get token-url` prints the active context resource-server OAuth2 token URL.
89. `declarest resource-server get access-token` prints only the OAuth2 access token for the active context resource server.
90. `declarest resource-server check` reports a successful connectivity probe even when the root probe returns `NotFoundError` because the server was reached.
