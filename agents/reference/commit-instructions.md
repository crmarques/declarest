# Commit Instructions

## Purpose
Own the commit subject-line format used for final handoff and explicit commit messages.

## Normative Rules
1. A commit message MUST be a single subject line in Conventional Commits form: `<type>(<scope>): <description>` — no body and no trailers (including `Co-Authored-By`).
2. The subject MUST be <= 72 characters.
3. `<type>` MUST be one of: `feat`, `fix`, `docs`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
4. `<scope>` SHOULD name the affected package/module/folder when obvious, and MAY be omitted otherwise.
5. A standard successful handoff MUST emit ONLY the subject line: no body, summaries, file lists, verification details, or commit questions.
6. When request processing is blocked or required verification cannot complete, the agent MUST report the blocker instead of emitting a subject line.

## Examples
- Success: `docs(agents): shorten standard handoff`
- Blocked: `Blocked: go test -race ./... could not complete`
