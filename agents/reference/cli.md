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

## Data Contracts
Command groups:
1. Basic Commands: `ad-hoc`, `config`, `metadata`, `repo`, `resource`, `secret`.
2. Other Commands: `completion`, `version`.

Global flags:
1. `--context`, `-c`.
2. `--debug`, `-d`.
3. `--no-status`, `-n`.
4. `--output`, `-o` with allowed formats `auto|text|json|yaml`.

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
1. `config use`.
2. `config current`.
3. `config resolve`.
4. `metadata resolve`.
5. `metadata render`.
6. `repo status`.
7. `secret mask`.
8. `secret resolve`.
9. `secret normalize`.
10. `secret detect`.
11. `completion`.
12. `version`.

## CLI Input Grammar
1. Resource targets MUST be logical absolute paths.
2. Metadata targets accept collection and resource scopes with positional path and `--path`.
3. Mutations from stdin MUST validate payload format before side effects.
4. Option conflicts MUST produce usage errors.
5. `resource list` MUST support `--recursive` and default to non-recursive direct-child listing.
6. `resource delete` MUST support `--recursive` and default to non-recursive collection deletes.

## Output Contract
1. Success output MAY be human-readable by default.
2. Structured output mode MUST define stable keys for automation.
3. Error output MUST include category and actionable next step where possible.
4. Diff output MUST present deterministic ordering.
5. When `--output` is `auto` (default), resource-oriented output MUST follow the active context `repository.resource-format` (`json` or `yaml`).
6. `repo status` with `--output auto` MUST render deterministic text summary by default.

## Failure Modes
1. Missing required path argument.
2. Invalid payload format.
3. Unsupported command/flag combination.
4. Command requires configured manager not present in active context.

## Edge Cases
1. `save` with secret masking requested but no secret manager configured.
2. `delete` invoked on collection without recursive force confirmation.
3. `metadata infer` called with missing OpenAPI source.
4. Completion for alias path when remote ID differs.

## Examples
1. `declarest resource apply /customers/acme` applies desired state for one resource.
2. `declarest resource apply --path /customers/acme` applies desired state for one resource using flag input.
3. `declarest metadata infer --path /customers --apply --recursive` writes inferred metadata recursively.
4. `declarest metadata render /customers/acme get` renders metadata operation spec.
5. `declarest repo push --force` executes force push with explicit safety acknowledgment.
6. `declarest repo status` reports local/remote sync status without mutating repository state.
7. `declarest completion bash` generates Bash completion output.
8. `declarest version -o json` prints machine-readable version information.
