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

## Data Contracts
Command groups:
1. Basic Commands: `ad-hoc`, `config`, `metadata`, `repo`, `resource`, `secret`.
2. Other Commands: `completion`, `version`.

Global flags:
1. `--context`, `-c`.
2. `--debug`, `-d`.
3. `--no-status`, `-n`.
4. `--output`, `-o` with allowed formats `auto|text|json|yaml`.
5. `--help`, `-h`.

Input flags:
1. `--file`, `-f`.
2. `--format`, `-i` with allowed formats `json|yaml`.

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
2. `config add`.
3. `config use`.
4. `config show`.
5. `config current`.
6. `config resolve`.
7. `metadata resolve`.
8. `metadata render`.
9. `repo status`.
10. `secret mask`.
11. `secret resolve`.
12. `secret normalize`.
13. `secret detect`.
14. `completion`.
15. `version`.

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
2. `config use` SHOULD support context selection when no name argument is provided.
3. `config show` SHOULD support context selection when `--context` is omitted.
4. `config rename` SHOULD support context selection and target-name prompt when arguments are omitted.
5. `config delete` SHOULD support context selection and explicit confirmation when no name argument is provided.

## CLI Input Grammar
1. Resource targets MUST be logical absolute paths.
2. Metadata targets accept collection and resource scopes with positional path and `--path`.
3. Mutations from stdin MUST validate payload format before side effects.
4. Option conflicts MUST produce usage errors.
5. `resource get` MUST support mutually exclusive `--local` and `--remote` flags.
6. `resource get` MUST default to `--remote` when neither source flag is provided.
7. `resource save` MUST support mutually exclusive `--as-items` and `--as-one-resource` flags.
8. `resource save` MUST default to `--as-items` behavior when input payload is a list (`[]` or object with `items` array).
9. `resource save` MUST reject potential plaintext secret values and fail with `ValidationError` unless `--insecure` is set.
10. `secret detect` MUST support optional `--fix` to persist detected attributes into metadata `secretsFromAttributes`.
11. `secret detect` without input payload (`--file` or stdin) MUST scan local repository resources recursively under positional `<path>`/`--path`, defaulting to `/` when path is omitted.
12. `secret detect --fix` in input-payload mode MUST require a target path from positional `<path>` or `--path`.
13. `secret detect --fix` in repository-scan mode MUST merge detected attributes into metadata `secretsFromAttributes` for each detected resource path in scope.
14. `secret detect --secret-attribute <attr>` MUST apply only that detected attribute and MUST fail with `ValidationError` when the requested attribute is not detected in payload or repository scope.
15. `resource list` MUST support `--recursive` and default to non-recursive direct-child listing.
16. `resource delete` MUST support `--recursive` and default to non-recursive collection deletes.
17. `resource apply` MUST treat collection paths as batch targets resolved from local repository resources and default to non-recursive direct-child execution.
18. `resource apply --recursive` MUST include descendant resources under the target path.
19. `resource create` and `resource update` with explicit payload input (`--file` or stdin) MUST execute a single-resource mutation.
20. `resource create` and `resource update` without payload input MUST load local repository payloads for all resources under the target path and mutate each matching resource.
21. `resource create` and `resource update` without `--recursive` MUST mutate only direct-child resources for collection paths.
22. `resource create --recursive` and `resource update --recursive` without payload input MUST include descendant resources under the target path.
23. `resource create` and `resource update` MUST fail with `ValidationError` when `--recursive` is provided together with explicit payload input.
24. Interactive config flows MUST fail fast with `ValidationError` when invoked without required arguments in non-interactive environments.
25. `config show` MUST use `--context` when provided and otherwise require interactive context selection.
26. `ad-hoc <method>` MUST accept endpoint path from positional `<path>` and `--path`, and mismatched values MUST fail with `ValidationError`.
27. `ad-hoc <method>` MUST accept optional request payload from `--file` or stdin, decoding according to `--format` (`json|yaml`) when payload input is provided.
28. `config add` MUST accept input from `--file` or stdin.
29. `config add` MUST accept either one `context` object or one full `contexts.yaml` catalog object.
30. When `config add` receives a catalog input and `--context-name` is omitted, it MUST import all catalog contexts.
31. When `config add` receives a catalog input and `--context-name` is set, it MUST import only the matching catalog context name.
32. When `config add` receives a single context object and `--context-name` is set, the imported context name MUST be overridden by `--context-name`.
33. `config add --set-current` MUST set current context to the imported context when exactly one context is imported.
34. `config add --set-current` with multiple imported contexts MUST require catalog `current-ctx` or fail with `ValidationError`.
35. `config add` SHOULD default `--format` to `yaml` while continuing to accept explicit `json`.
36. Help and completion-script invocations MUST bypass context-dependent startup validation so command usage remains available when no current context is configured.
37. Command-group invocations without subcommands MUST bypass context-dependent startup validation so usage/help output remains available when no current context is configured.

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

## Failure Modes
1. Missing required path argument.
2. Invalid payload format.
3. Unsupported command/flag combination.
4. Command requires configured manager not present in active context.
5. `resource get` receives both `--local` and `--remote`.
6. `resource save` receives both `--as-items` and `--as-one-resource`.
7. `resource save --as-items` receives non-list input.
8. `resource save` detects potential plaintext secret values and `--insecure` is not set.
9. `secret detect --fix` is provided with payload input but without path input.
10. `secret detect --secret-attribute` value is not detected in payload or repository scope.
11. `config add --context-name` does not match any catalog context.
12. `config add --set-current` with multiple imported contexts and missing catalog `current-ctx`.
13. `resource create` or `resource update` receives `--recursive` together with explicit payload input.
14. `resource apply`, `resource create`, or `resource update` targets a collection path with no local resources.

## Edge Cases
1. `save` with secret masking requested but no secret manager configured.
2. `delete` invoked on collection without recursive force confirmation.
3. `metadata infer` called with missing OpenAPI source.
4. Completion for alias path when remote ID differs.
5. Interactive config command invoked from non-TTY input/output.
6. `resource save --help` invoked when no current context exists.
7. `completion` or shell completion engine invocation (`__complete`, `__completeNoDesc`) when no current context exists.
8. `resource save` payload mixes placeholders and plaintext values for secret-like fields.
9. Root command completion includes command help entry as `help` and excludes internal aliases.
10. `secret detect --fix` in repository-scan mode updates metadata for paths that currently have no metadata files.
11. `resource` invoked without a subcommand when no current context exists.
12. `resource apply` is invoked on a collection that has only nested descendants and omits `--recursive`.

## Examples
1. `declarest resource apply /customers/acme` applies desired state for one resource.
2. `declarest resource apply --path /customers/acme` applies desired state for one resource using flag input.
3. `declarest resource apply /customers` applies all direct-child local resources in `/customers`.
4. `declarest resource apply /customers --recursive` applies direct and nested resources under `/customers`.
5. `declarest resource create /customers --recursive` loads local payloads from repository files and creates every matched remote resource.
6. `declarest resource update /customers` updates only direct-child resources in `/customers` and skips nested descendants.
7. `declarest resource create /customers/acme --file payload.json` creates one resource from explicit payload input.
8. `declarest resource update /customers/acme --recursive --file payload.json` fails with `ValidationError` because `--recursive` conflicts with explicit input mode.
9. `declarest resource get /customers/acme` reads remote state by default.
10. `declarest resource get /customers/acme --local` reads local repository state.
11. `declarest resource save /customers < list.json` stores each list item as its own resource when `list.json` is a list payload.
12. `declarest resource save /customers --as-one-resource < list.json` stores the list payload in one resource file.
13. `declarest resource save /customers/acme < payload.json` fails with `ValidationError` when plaintext secret candidates are detected.
14. `declarest resource save /customers/acme --insecure < payload.json` bypasses plaintext-secret save guard.
15. `declarest secret detect /customers/acme --fix < payload.json` detects secret attributes and writes them to metadata `secretsFromAttributes` for `/customers/acme`.
16. `declarest secret detect /customers/acme --fix --secret-attribute password < payload.json` writes only `password` from detected candidates.
17. `declarest metadata infer --path /customers --apply --recursive` writes inferred metadata recursively.
18. `declarest metadata render /customers/acme get` renders metadata operation spec.
19. `declarest repo push --force` executes force push with explicit safety acknowledgment.
20. `declarest repo status` reports local/remote sync status without mutating repository state.
21. `declarest completion bash` generates Bash completion output.
22. `declarest version -o json` prints machine-readable version information.
23. `declarest config use` opens interactive context selection when run in a terminal.
24. `declarest config show --context dev` prints the selected context configuration as YAML.
25. `declarest ad-hoc get /health` executes a direct managed-server GET request.
26. `declarest ad-hoc post /customers --file payload.json` executes a direct managed-server POST request with JSON body.
27. `echo '{"id":"acme"}' | declarest ad-hoc put /customers/acme` executes a direct managed-server PUT request from stdin.
28. `declarest ad-hoc delete /customers/a --path /customers/b` fails with `ValidationError` due to path mismatch.
29. `declarest config create` opens interactive prompts to build one context configuration.
30. `declarest config add --file contexts.yaml --format yaml` imports all contexts defined in a catalog file.
31. `declarest config add --file contexts.yaml --format yaml --context-name prod --set-current` imports only `prod` and sets it as current.
32. `declarest config add --file contexts.yaml --format yaml --set-current` fails when multiple contexts are imported and the catalog omits `current-ctx`.
33. `declarest resource save --help` prints help text even when no current context is configured.
34. `declarest secret detect` scans the whole local repository for secret candidates when no payload input is provided.
35. `declarest secret detect /customers --fix` scans local resources under `/customers` and updates metadata `secretsFromAttributes` for detected resource paths.
36. `declarest completion bash` prints completion script even when no current context is configured.
37. `declarest` shell tab completion at root suggests `help` and does not suggest internal helper names.
38. `declarest resource` prints resource command help even when no current context is configured.
