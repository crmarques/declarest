Use Conventional Commits: <type>(<scope>): <description>

Generate ONLY one short subject line (no body). Max 72 chars

For a successful standard request handoff, output ONLY that one subject line.
Do NOT append summaries, file lists, verification details, or commit questions.
If request processing is blocked or required verification cannot complete, report the blocker instead.

Allowed types: feat, fix, docs, refactor, perf, test, build, ci, chore, revert

Use a scope when obvious (package/module/folder)

Examples:
- Success: `docs(agents): shorten standard handoff`
- Blocked corner case: `Blocked: go test -race ./... could not complete`
