# AGENTS

## Purpose
This repository uses an AGENTS document to orient AI contributors. Whenever you start work:

- Read this file to understand the project expectations, tooling, and helpful patterns.
- Confirm whether other skill-specific instructions are active in `~/.codex/skills` and apply them as needed.

## Guidelines for AI contributors

1. **Prioritize the repository requirements.** Always cross-check your changes against `specs/specs.md` (adjusted version) and the command-line user expectations documented in `docs/reference/cli.md`.
2. **Lean on the existing architecture.** Make minimal changes to interfaces in `internal/` packages; follow the module layering described in `specs/specs.md` §6.
3. **Follow standard workflows.** Prefer `go test ./...` and `gofmt` for Go files, ensure shell scripts pass `shellcheck` when practical, and use `rg` for fast lookups.
4. **Document behavior.** Add coverage or notes for new features, especially when introducing CLI flags or metadata semantics.
5. **Escalate blockers.** If you encounter runtime/environment constraints (e.g., Podman permission errors), describe them in the final report rather than rerunning failing commands indefinitely.

## Update protocol

- When you edit this file, note what new expectations future agents should know.
- Keep guidance concise; AGENTS is meant to be quick to read before coding.
