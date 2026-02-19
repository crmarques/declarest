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
1. CLI commands MUST map directly to reconciler use cases.
2. Input validation MUST fail fast with usage guidance and non-zero exit codes.
3. Destructive operations MUST require explicit confirmation flag or equivalent safety gate.
4. Output for machine parsing MUST be stable and documented.
5. Human-readable output SHOULD be concise and deterministic.
6. Command aliases MUST not introduce ambiguous behavior.
7. Completion suggestions MUST be context-aware and deterministic.
8. Interactive command flows MUST only run when stdin/stdout are interactive terminals and MUST preserve non-interactive automation behavior.
9. Help invocations (`--help`, `-h`, or `help`) and completion-script invocations (`completion`, `__complete`, `__completeNoDesc`) MUST render without requiring active-context resolution.
10. Shell completion output MUST expose canonical command names and MUST NOT leak internal command placeholders.
11. Invoking a command group without a required subcommand MUST render that group's help and MUST NOT require active-context resolution.
12. Non-runtime commands (`version`, `config create|print-template|add|update|delete|rename|list|use|show|current|resolve|validate`) MUST execute without requiring active-context resolution at startup.
13. When `--repo-type git` is selected and no `--git-provider` is supplied, the CLI MUST default the provider to the local `git` component so git-backed repositories integrate without additional flags while still enforcing explicit overrides when provided.
14. Path completion MUST merge repository paths, remote resource paths, and OpenAPI paths; for templated OpenAPI segments (`{...}`), completion SHOULD resolve concrete candidates by listing local and remote collection children with metadata-aware path semantics.

## Data Contracts
Command groups:
1. Basic Commands: `ad-hoc`, `config`, `metadata`, `repo`, `resource`, `secret`.
2. Other Commands: `completion`, `version`.

Global flags:
1. `--context`, `-c`.
2. `--debug`, `-d`.
3. `--verbose`, `-v`.
4. `--no-status`, `-n`.
5. `--output`, `-o` with allowed formats `auto|text|json|yaml`.
6. `--help`, `-h`.

Input flags:
1. `--file`, `-f`.
2. `--format`, `-i` with allowed formats `json|yaml`.
3. `--payload` as a command-specific inline payload flag for `ad-hoc post`, `ad-hoc put`, and `resource create`.

Path flags:
1. Path-aware commands MUST accept `--path`, `-p`.
2. Path-aware commands MUST also accept positional `<path>`.
3. If both positional path and `--path` are provided and values differ, the command MUST fail with `ValidationError`.

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

Selected command names:
1. `config create`.
2. `config print-template`.
3. `config add`.
4. `config use`.
5. `config show`.
6. `config current`.
7. `config resolve`.
8. `metadata resolve`.
9. `metadata render`.
10. `repo status`.
11. `secret mask`.
12. `secret resolve`.
13. `secret normalize`.
14. `secret detect`.
15. `completion`.
16. `version`.

Ad-hoc command methods:
1. `ad-hoc get`.
2. `ad-hoc head`.
3. `ad-hoc options`.
4. `ad-hoc post`.
5. `ad-hoc put`.
6. `ad-hoc patch`.
7. `ad-hoc delete`.
8. `ad-hoc trace`.
9. `ad-hoc connect`.

Interactive config commands:
1. `config create` SHOULD support terminal prompts when no file/stdin input is provided.
2. `config create` SHOULD accept optional context name from positional `[new-context-name]` or global `--context` and skip name prompting when provided.
3. `config use` SHOULD support context selection when no name argument is provided.
4. `config show` SHOULD support context selection when `--context` is omitted.
5. `config rename` SHOULD support context selection and target-name prompt when arguments are omitted.
6. `config delete` SHOULD support context selection and explicit confirmation when no name argument is provided.
7. `config create` SHOULD surface optional sections with explicit skip choices and, for one-of blocks, SHOULD prompt only the selected branch fields.

## CLI Input Grammar
1. Resource targets MUST be logical absolute paths.
2. Metadata targets accept collection and resource scopes with positional path and `--path`.
3. Mutations from stdin MUST validate payload format before side effects.
4. Option conflicts MUST produce usage errors.
5. `resource get` MUST support mutually exclusive `--repository` and `--remote-server` flags.
6. `resource get` MUST default to `--remote-server` when neither source flag is provided, and remote reads MUST attempt the literal path first then list/filter by metadata-derived identity when the literal read returns `NotFound`.
7. `resource list` MUST support mutually exclusive `--repository` and `--remote-server` flags.
8. `resource list` MUST default to `--remote-server` when neither source flag is provided.
9. `resource list` MUST support `--recursive` and default to non-recursive direct-child listing.
10. `resource delete` MUST support mutually exclusive `--repository`, `--remote-server`, and `--both` flags and default to `--remote-server` when no source flag is provided.
11. `resource delete` MUST support `--recursive` and default to non-recursive collection deletes.
12. `resource apply` MUST treat collection paths as batch targets resolved from local repository resources and default to non-recursive direct-child execution.
13. `resource apply --recursive` MUST include descendant resources under the target path.
14. `resource create` MUST accept explicit payload input (`--file`, stdin, or `--payload`) for a single remote mutation, and when payload input is absent it MUST load local repository payloads for resources under the target path and execute create for each resolved target.
15. `resource update` MUST load local repository payloads for resources under the target path and execute remote updates for each matching resource.
16. `resource update` without `--recursive` MUST mutate only direct-child resources for collection paths.
17. `resource update --recursive` MUST include descendant resources under the target path.
18. `resource save` without payload input (`--file` or stdin) MUST read the requested path from the remote server and persist the value into the repository, using the same literal-then-list/filter metadata-aware fallback as `resource get`.
19. `resource save` MUST support mutually exclusive `--as-items` and `--as-one-resource` flags.
20. `resource save` MUST default to `--as-items` behavior when input payload is a list (`[]` or object with `items` array).
21. `resource save` MUST reject potential plaintext secret values and fail with `ValidationError` unless `--ignore` or `--handle-secrets` is set; if the logical path already exists in the repository, overriding the persisted resource MUST additionally require `--force`.
22. `resource save --handle-secrets` MUST accept an optional comma-separated attribute list; when no list is provided, all detected plaintext secret candidates MUST be handled.
23. `resource save --handle-secrets` MUST detect plaintext secret attributes, store handled values in the configured secret store using path-scoped keys, replace handled payload values with `{{secret .}}` placeholders, and merge handled attributes into metadata `secretsFromAttributes` for the saved logical path.
24. Resource payload placeholder resolution for remote workflows MUST resolve `{{secret .}}` as `<logical-path>:<attribute-path>`, resolve `{{secret <custom-key>}}` as `<logical-path>:<custom-key>`, and remain compatible with legacy absolute key placeholders.
25. When `resource save --handle-secrets` handles only a subset of detected candidates, the command MUST fail with the same plaintext-secret warning using only unhandled candidates unless `--ignore` is set.
26. For collection list saves (`--as-items` default), plaintext-secret candidate detection MUST be computed once per save from the collection payload set and then applied consistently across all list items.
27. `secret detect` MUST support optional `--fix` to persist detected attributes into metadata `secretsFromAttributes`.
28. `secret detect` without input payload (`--file` or stdin) MUST scan local repository resources recursively under positional `<path>`/`--path`, defaulting to `/` when path is omitted.
29. `secret detect --fix` in input-payload mode MUST require a target path from positional `<path>` or `--path`.
30. `secret detect --fix` in repository-scan mode MUST merge detected attributes into metadata `secretsFromAttributes` for each detected resource path in scope.
31. `secret detect --secret-attribute <attr>` MUST apply only that detected attribute and MUST fail with `ValidationError` when the requested attribute is not detected in payload or repository scope.
32. Interactive config flows MUST fail fast with `ValidationError` when invoked without required arguments in non-interactive environments.
33. `config show` MUST use `--context` when provided and otherwise require interactive context selection.
34. `config create` MUST default `--format` to `yaml` while continuing to accept explicit `json`.
35. `config create` MUST accept optional context name from positional `[new-context-name]` or global `--context`.
36. `config create` MUST fail with `ValidationError` when positional `[new-context-name]` and global `--context` are both provided with different values.
37. `ad-hoc <method>` MUST accept endpoint path from positional `<path>` and `--path`, and mismatched values MUST fail with `ValidationError`; `ad-hoc get` MUST attempt metadata-aware remote read fallback when the literal ad-hoc request returns `NotFound`.
38. `ad-hoc <method>` MUST accept optional request payload from `--file` or stdin, decoding according to `--format` (`json|yaml`) when payload input is provided.
39. `ad-hoc post` and `ad-hoc put` MUST also support optional `--payload` inline input, decoded according to `--format`, and `--payload` MUST be mutually exclusive with `--file` and stdin input.
40. `config add` MUST accept input from `--file` or stdin.
41. `config add` MUST accept either one `context` object or one full `contexts.yaml` catalog object.
42. When `config add` receives a catalog input and `--context-name` is omitted, it MUST import all catalog contexts.
43. When `config add` receives a catalog input and `--context-name` is set, it MUST import only the matching catalog context name.
44. When `config add` receives a single context object and `--context-name` is set, the imported context name MUST be overridden by `--context-name`.
45. `config add --set-current` MUST set current context to the imported context when exactly one context is imported.
46. `config add --set-current` with multiple imported contexts MUST require catalog `current-ctx` or fail with `ValidationError`.
47. `config add` SHOULD default `--format` to `yaml` while continuing to accept explicit `json`.
48. Help and completion-script invocations MUST bypass context-dependent startup validation so command usage remains available when no current context is configured.
49. Command-group invocations without subcommands MUST bypass context-dependent startup validation so usage/help output remains available when no current context is configured.
50. `ad-hoc delete` MUST require `--force` and fail with `ValidationError` when confirmation is not explicit.
51. Repository-backed single-resource reads (`resource get --repository`, `resource apply`, `resource update`, `resource diff`, `resource explain`) MUST attempt literal repository lookup first and, on `NotFound`, perform a bounded collection fallback that matches by metadata `idFromAttribute`.
52. `resource apply|create|update` collection-target resolution MUST attempt a non-recursive collection list first and, when no entries match a deep path target, attempt single-resource fallback lookup before returning `NotFound`.
53. `resource delete --remote-server` MUST resolve collection targets from local repository resources (direct-child by default, descendants with `--recursive`) and, when no local targets match, attempt literal delete with metadata-aware remote identity fallback on `NotFound`.
54. `ad-hoc delete` MUST resolve collection targets from local repository resources (direct-child by default, descendants with `--recursive`) and issue one delete request per resolved target; when no local targets match it MUST issue a single delete request for the requested path.
55. `resource save` MUST accept `_` as a wildcard path segment when no payload input is provided and MUST expand each wildcard level through remote direct-child list lookups before saving resolved targets.
56. `resource save` with wildcard path segments and payload input (`--file` or stdin) MUST fail with `ValidationError`.
57. `resource save` wildcard expansions for resource targets MUST skip unresolved concrete `NotFound` reads and MUST return `NotFoundError` when no concrete targets resolve successfully.
58. `resource diff` MUST resolve collection targets from local repository resources (direct-child by default), execute compare for each resolved resource, and when no collection targets match a deep path it MUST attempt single-resource fallback lookup before returning `NotFound`.
59. Interactive `config create` MUST support full context-schema authoring: prompt required fields for selected providers, offer skip paths for optional sections, and enforce one-of prompt branching (for example oauth2 vs basic-auth) by collecting only the selected option's fields.
60. `config print-template` MUST output a commented YAML context catalog template that includes all supported configuration branches and explicitly marks mutually-exclusive blocks.

## Output Contract
1. Success output MAY be human-readable by default.
2. Structured output mode MUST define stable keys for automation.
3. Error output MUST include category and actionable next step where possible.
4. Diff output MUST present deterministic ordering.
5. When `--output` is `auto` (default), resource-oriented output MUST follow the active context `repository.resource-format` (`json` or `yaml`).
6. `repo status` with `--output auto` MUST render deterministic text summary by default.
7. `config show` MUST print the full selected context configuration as YAML to stdout.
8. Command help output MUST present `--help` in the `Global Flags` section.
9. HTTP transport debug output MUST include TLS/mTLS configuration context (`tls_enabled`, `mtls_enabled`, and configured TLS file paths) without logging secret values.
10. Help output SHOULD avoid repeated blank lines between sections.
11. `resource diff --output text` MUST render one line per diff entry using relative dot-path notation from the requested target path and JSON-encoded values in the form `<dot-path> [Local=<json>] => [Remote=<json>]`.
12. Unless `--no-status` is set, resource-mutation commands (`resource save|apply|create|update|delete`) and all runnable `ad-hoc` method commands (`ad-hoc get|head|options|post|put|patch|delete|trace|connect`) MUST print a terminal status line as the final output line to stderr using `[OK] <description>.` on success and `[ERROR] <description>.` on failure.
13. Interactive terminal status output SHOULD render `[OK]` in bold green and `[ERROR]` in bold red.
14. Commands returning nil payload output MUST emit no payload body (no `null`/`<nil>` placeholder output).
15. State-changing commands (`resource save|apply|create|update|delete` and `ad-hoc post|put|patch|delete|connect`) MUST suppress complementary payload output by default and print only the status footer.
16. `--verbose` MUST re-enable complementary payload output for commands that suppress it by default.
17. `config check` text output MUST report component rows using `context`, `repository`, `metadata`, `resource-server`, and `secret-store` labels.

## Failure Modes
1. Missing required path argument.
2. Invalid payload format.
3. Unsupported command/flag combination.
4. Command requires configured manager not present in active context.
5. `resource get` receives both `--repository` and `--remote-server`.
6. `resource list` receives both `--repository` and `--remote-server`.
7. `resource delete` receives conflicting source flags (`--repository`, `--remote-server`, `--both`).
8. `resource save` receives both `--as-items` and `--as-one-resource`.
9. `resource save --as-items` receives non-list input.
10. `resource save` detects potential plaintext secret values and neither `--ignore` nor `--handle-secrets` is set, or the command attempts to overwrite an existing repository resource without `--force`.
11. `resource save --handle-secrets=<attr-list>` includes one or more attributes that are not detected in the payload.
12. `resource create` is invoked without payload input and no matching local resources exist under the target path.
13. `resource apply`, `resource create`, or `resource update` targets a collection path with no local resources.
14. `secret detect --fix` is provided with payload input but without path input.
15. `secret detect --secret-attribute` value is not detected in payload or repository scope.
16. `config add --context-name` does not match any catalog context.
17. `ad-hoc post` or `ad-hoc put` receives `--payload` together with `--file` or stdin input.
18. `resource create` receives `--payload` together with `--file` or stdin input.
19. `config add --set-current` with multiple imported contexts and missing catalog `current-ctx`.
20. `ad-hoc delete` is invoked without `--force`.
21. Metadata-aware identity fallback yields multiple candidates for the same requested path and returns `ConflictError`.
22. `resource save` wildcard path is combined with payload input.
23. `resource diff` targets a collection path with no local resources.
24. `config create` receives both positional context name and `--context` with different values.
25. `config print-template` receives positional arguments.

## Edge Cases
1. `resource save --handle-secrets` is requested but no secret manager is configured.
2. `resource save --handle-secrets` handles only a subset and fails with warning for the remaining plaintext candidates unless `--ignore` is set.
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
18. Completion for a templated OpenAPI path segment with a partial value (for example `/admin/realms/m`) returns concrete collection candidates when local or remote collection children are available and otherwise returns the template path candidate.
19. `version` and context-catalog management commands (for example `config list`) succeed when no current context is set, while runtime commands continue to fail fast when active context resolution is required.
20. `config create` with managed-server auth set to `oauth2` prompts only oauth2 fields and does not prompt `basic-auth`, `bearer-token`, or `custom-header` fields.
21. `config print-template` works without a configured current context and still renders the full template.

## Examples
1. `declarest resource apply /customers/acme` applies desired state for one resource.
2. `declarest resource apply --path /customers/acme` applies desired state for one resource using flag input.
3. `declarest resource apply /customers` applies all direct-child local resources in `/customers`.
4. `declarest resource apply /customers --recursive` applies direct and nested resources under `/customers`.
5. `declarest resource create /customers/acme --file payload.json` creates one remote resource from explicit payload input.
6. `declarest resource create /customers/acme --payload '{"id":"acme"}'` creates one remote resource from inline payload input.
7. `declarest resource create /customers` creates all direct-child resources in `/customers` using repository payloads.
8. `declarest resource update /customers` updates only direct-child resources in `/customers` using repository payloads and skips nested descendants.
9. `declarest resource update /customers --recursive` updates direct and nested resources under `/customers` using repository payloads.
10. `declarest resource get /customers/acme` reads remote state by default.
11. `declarest resource get /customers/acme --repository` reads local repository state.
12. `declarest resource get /customers --repository` lists repository resources under `/customers`, mirroring `declarest resource list /customers --repository`.
13. `declarest resource list /customers` lists remote resources by default.
14. `declarest resource list /customers --repository` lists repository resources.
15. `declarest resource delete /customers/acme --force` deletes from the remote server by default.
16. `declarest resource delete /customers/acme --force --both` deletes from both remote server and repository.
17. `declarest resource save /customers/acme` fetches remote state and saves it into repository for `/customers/acme`.
18. `declarest resource save /customers < list.json` stores each list item as its own resource when `list.json` is a list payload.
19. `declarest resource save /customers --as-one-resource < list.json` stores the list payload in one resource file.
20. `declarest resource save /customers/acme < payload.json` fails with `ValidationError` when plaintext secret candidates are detected.
21. `declarest resource save /customers/acme --ignore < payload.json` bypasses plaintext-secret save guard.
22. `declarest resource save /customers/acme --handle-secrets < payload.json` stores all detected secrets, masks payload values with placeholders, and updates metadata `secretsFromAttributes`.
23. `declarest resource save /customers/acme --handle-secrets=password < payload.json` handles only `password`; if other candidates remain, command fails with warning listing only the unhandled candidates unless `--ignore` is set.
24. `declarest secret detect /customers/acme --fix < payload.json` detects secret attributes and writes them to metadata `secretsFromAttributes` for `/customers/acme`.
25. `declarest secret detect /customers/acme --fix --secret-attribute password < payload.json` writes only `password` from detected candidates.
26. `declarest resource save /admin/realms/master/clients/` saves remote list items using metadata identity attributes and falls back to common attributes like `id` when metadata attributes are absent in payload entries.
27. `declarest metadata infer --path /customers --apply --recursive` writes inferred metadata recursively.
28. `declarest metadata render /customers/acme get` renders metadata operation spec.
29. `declarest repo push --force` executes force push with explicit safety acknowledgment.
30. `declarest repo status` reports local/remote sync status without mutating repository state.
31. `declarest completion bash` generates Bash completion output.
32. `declarest version -o json` prints machine-readable version information.
33. `declarest config use` opens interactive context selection when run in a terminal.
34. `declarest config show --context dev` prints the selected context configuration as YAML.
35. `declarest ad-hoc get /health` executes a direct managed-server GET request.
36. `declarest ad-hoc post /customers --file payload.json` executes a direct managed-server POST request with JSON body.
37. `declarest ad-hoc post /customers --payload '{"id":"acme"}'` executes a direct managed-server POST request with inline JSON payload.
38. `echo '{"id":"acme"}' | declarest ad-hoc put /customers/acme` executes a direct managed-server PUT request from stdin.
39. `declarest ad-hoc delete /customers/a --path /customers/b` fails with `ValidationError` due to path mismatch.
40. `declarest config create` opens interactive prompts to build one context configuration.
41. `declarest config add --file contexts.yaml --format yaml` imports all contexts defined in a catalog file.
42. `declarest config add --file contexts.yaml --format yaml --context-name prod --set-current` imports only `prod` and sets it as current.
43. `declarest config add --file contexts.yaml --format yaml --set-current` fails when multiple contexts are imported and the catalog omits `current-ctx`.
44. `declarest resource save --help` prints help text even when no current context is configured.
45. `declarest secret detect` scans the whole local repository for secret candidates when no payload input is provided.
46. `declarest secret detect /customers --fix` scans local resources under `/customers` and updates metadata `secretsFromAttributes` for detected resource paths.
47. `declarest completion bash` prints completion script even when no current context is configured.
48. `declarest` shell tab completion at root suggests `help` and does not suggest internal helper names.
49. `declarest resource` prints resource command help even when no current context is configured.
50. `declarest ad-hoc delete /customers/acme` fails with `ValidationError` because `--force` is required.
51. `declarest ad-hoc delete /customers/acme --force` executes a direct managed-server DELETE request.
52. `declarest ad-hoc delete /customers --force --recursive` issues delete requests for all repository resources under `/customers`.
53. `declarest resource apply /admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c` applies the local client resource whose metadata `idFromAttribute` matches the provided path segment when no literal repository resource exists.
54. `declarest resource delete /admin/realms/master/clients/account --force --remote-server` retries deletion using metadata-resolved remote ID when the literal delete path is not found.
55. `declarest resource save /admin/realms/_/clients/` expands wildcard realms and saves clients from all matched realms.
56. `declarest resource save /admin/realms/_/clients/test` expands wildcard realms and saves each matched `test` client resource path.
57. `declarest resource diff /customers` compares all direct-child repository resources in `/customers` and returns a single deterministic diff list.
58. `declarest resource diff /admin/realms/master/clients/f88c68f3-3253-49f9-94a9-fe7553d33b5c` falls back to single-resource lookup when collection resolution for that deep path has no direct matches.
59. `declarest resource diff /admin/realms/payments --output text` prints lines like `.displayName [Local="Payments Realm"] => [Remote="Payments Realm 2"]`.
60. `declarest resource save /customers/acme -f payload.json -i json` terminates with `[OK] command executed successfully.`.
61. `declarest resource save /customers/acme -f payload.json -i json --no-status` suppresses the final status line.
62. `declarest ad-hoc get /health` terminates with `[OK] command executed successfully.`.
63. `declarest resource create /customers/acme --payload '{"id":"acme"}'` prints no payload output by default and only the final status footer.
64. `declarest resource create /customers/acme --payload '{"id":"acme"}' --verbose` prints the created target payload output plus the final status footer.
65. `declarest ad-hoc delete /customers --force --recursive` prints no response bodies by default and only the final status footer.
66. `declarest ad-hoc delete /customers --force --recursive --verbose` prints response bodies for each resolved delete target plus the final status footer.
67. `declarest resource get /admin/realms/m<TAB>` completes to concrete candidates such as `/admin/realms/master` by combining OpenAPI templates with local/remote collection item lookups.
68. `declarest config create dev` skips context-name prompt and starts interactive prompts at repository settings.
69. `declarest config create --context dev` skips context-name prompt and starts interactive prompts at repository settings.
70. `declarest config create full` can populate managed-server, secret-store, TLS, and preference fields interactively while allowing optional sections to be skipped.
71. `declarest config print-template` prints a full commented `contexts.yaml` template including mutually-exclusive option guidance.
