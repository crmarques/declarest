# AGENTS

## Purpose
This repository uses an AGENTS document to orient AI contributors. Whenever you start work:

- Read this file to understand the project expectations, tooling, and helpful patterns.
- Confirm whether other skill-specific instructions are active in `~/.codex/skills` and apply them as needed.

## Guidelines for AI contributors

1. **Prioritize the repository requirements.** Identify the domain/concern of the task, then load the relevant spec file(s) from `specs/` plus `docs/reference/cli.md` when CLI behavior is involved. Use `specs/specs.md` as the full source of truth.
2. **Lean on the existing architecture.** Make minimal changes to interfaces in `internal/` packages; follow the module layering described in `specs/specs.md` §6.
3. **Follow standard workflows.** Prefer `go test ./...` and `gofmt` for Go files, ensure shell scripts pass `shellcheck` when practical, and use `rg` for fast lookups.
4. **Document behavior.** Add coverage or notes for new features, especially when introducing CLI flags or metadata semantics.
5. **Escalate blockers.** If you encounter runtime/environment constraints (e.g., Podman permission errors), describe them in the final report rather than rerunning failing commands indefinitely.
6. **Close the feedback loop on failures.** When an error is reported, analyze the logs, implement the fix, and rerun the requested tests (e.g., the e2e command) until they finish successfully before stopping.
7. **Respect existing architecture and formats.** Any new code should mirror the structural and coding patterns used by similar, already-implemented components, and input/output files must follow the established formats or outputs currently in use.
8. **Avoid explanatory comments.** Keep code self-documenting, and do not add or preserve comments that merely restate what the code already expresses.

## Domain specs (load as needed)
- `specs/01-purpose-glossary.md`: purpose and shared terminology.
- `specs/02-repo-layout.md`: logical path normalization and on-disk layout.
- `specs/03-metadata.md`: metadata layering, defaults, and resolution rules.
- `specs/04-cli.md`: CLI behavior and command semantics.
- `specs/05-architecture.md`: module boundaries and ownership.
- `specs/06-contexts.md`: context/config behavior.
- `specs/07-quality-security.md`: quality gates, error handling, and security invariants.
- `specs/agents.md`: completion behavior for CLI.

## Update protocol

- When you edit this file, note what new expectations future agents should know.
- Keep guidance concise; AGENTS is meant to be quick to read before coding.
- Document user or developer-facing behavior changes in the docs before finishing your work.
- When new feature is added, update unit and e2e tests to validade it
- Latest update: split `specs/specs.md` into domain-focused files and require loading the relevant spec file(s) per task.
