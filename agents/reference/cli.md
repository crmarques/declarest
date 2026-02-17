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
9. `resource list` MUST support `--recursive` and default to non-recursive direct-child listing.
10. `resource delete` MUST support `--recursive` and default to non-recursive collection deletes.
11. Interactive config flows MUST fail fast with `ValidationError` when invoked without required arguments in non-interactive environments.
12. `config show` MUST use `--context` when provided and otherwise require interactive context selection.
13. `ad-hoc <method>` MUST accept endpoint path from positional `<path>` and `--path`, and mismatched values MUST fail with `ValidationError`.
14. `ad-hoc <method>` MUST accept optional request payload from `--file` or stdin, decoding according to `--format` (`json|yaml`) when payload input is provided.
15. `config add` MUST accept input from `--file` or stdin.
16. `config add` MUST accept either one `context` object or one full `contexts.yaml` catalog object.
17. When `config add` receives a catalog input and `--context-name` is omitted, it MUST import all catalog contexts.
18. When `config add` receives a catalog input and `--context-name` is set, it MUST import only the matching catalog context name.
19. When `config add` receives a single context object and `--context-name` is set, the imported context name MUST be overridden by `--context-name`.
20. `config add --set-current` MUST set current context to the imported context when exactly one context is imported.
21. `config add --set-current` with multiple imported contexts MUST require catalog `current-ctx` or fail with `ValidationError`.
22. `config add` SHOULD default `--format` to `yaml` while continuing to accept explicit `json`.

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

## Failure Modes
1. Missing required path argument.
2. Invalid payload format.
3. Unsupported command/flag combination.
4. Command requires configured manager not present in active context.
5. `resource get` receives both `--local` and `--remote`.
6. `resource save` receives both `--as-items` and `--as-one-resource`.
7. `resource save --as-items` receives non-list input.
8. `config add --context-name` does not match any catalog context.
9. `config add --set-current` with multiple imported contexts and missing catalog `current-ctx`.

## Edge Cases
1. `save` with secret masking requested but no secret manager configured.
2. `delete` invoked on collection without recursive force confirmation.
3. `metadata infer` called with missing OpenAPI source.
4. Completion for alias path when remote ID differs.
5. Interactive config command invoked from non-TTY input/output.

## Examples
1. `declarest resource apply /customers/acme` applies desired state for one resource.
2. `declarest resource apply --path /customers/acme` applies desired state for one resource using flag input.
3. `declarest resource get /customers/acme` reads remote state by default.
4. `declarest resource get /customers/acme --local` reads local repository state.
5. `declarest resource save /customers < list.json` stores each list item as its own resource when `list.json` is a list payload.
6. `declarest resource save /customers --as-one-resource < list.json` stores the list payload in one resource file.
7. `declarest metadata infer --path /customers --apply --recursive` writes inferred metadata recursively.
8. `declarest metadata render /customers/acme get` renders metadata operation spec.
9. `declarest repo push --force` executes force push with explicit safety acknowledgment.
10. `declarest repo status` reports local/remote sync status without mutating repository state.
11. `declarest completion bash` generates Bash completion output.
12. `declarest version -o json` prints machine-readable version information.
13. `declarest config use` opens interactive context selection when run in a terminal.
14. `declarest config show --context dev` prints the selected context configuration as YAML.
15. `declarest ad-hoc get /health` executes a direct managed-server GET request.
16. `declarest ad-hoc post /customers --file payload.json` executes a direct managed-server POST request with JSON body.
17. `echo '{"id":"acme"}' | declarest ad-hoc put /customers/acme` executes a direct managed-server PUT request from stdin.
18. `declarest ad-hoc delete /customers/a --path /customers/b` fails with `ValidationError` due to path mismatch.
19. `declarest config create` opens interactive prompts to build one context configuration.
20. `declarest config add --file contexts.yaml --format yaml` imports all contexts defined in a catalog file.
21. `declarest config add --file contexts.yaml --format yaml --context-name prod --set-current` imports only `prod` and sets it as current.
22. `declarest config add --file contexts.yaml --format yaml --set-current` fails when multiple contexts are imported and the catalog omits `current-ctx`.
