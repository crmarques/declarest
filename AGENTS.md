# AGENTS

## Purpose
Define how coding agents operate in this repository rebuild. Canonical references are in `agents/reference/`; reusable workflows are in `agents/skills/`.

## Startup Protocol
1. Read `AGENTS.md`.
2. Identify request intent and affected bounded contexts.
3. Load `agents/reference/interfaces.md` first.
4. Run `agents/skills/spec-router/SKILL.md` and load the minimal additional files from the matrix below.
5. If authoring/revising specs or instruction files, run `agents/skills/spec-writer/SKILL.md`.
6. If behavior or verification expectations changed, run `agents/skills/quality-gate/SKILL.md`.
7. If reviewing/auditing specs, or after substantial spec/instruction edits, run `agents/skills/spec-auditor/SKILL.md`.
8. Before final response, run the completion checklist and report unmet items.

## Domain File Catalog
| File | Domain | Load When |
|---|---|---|
| `agents/reference/interfaces.md` | Canonical contracts | Always |
| `agents/reference/architecture.md` | Boundaries and dependency rules | Designing components, refactors |
| `agents/reference/code.md` | Code patterns and implementation standards | Implementing or reviewing code |
| `agents/reference/domain.md` | Vocabulary and invariants | Modeling behavior and data |
| `agents/reference/context-config.md` | Context and config semantics | Context loading, overrides, validation |
| `agents/reference/resource-repo.md` | Resource repository and Git/FS semantics | Storage, sync, path handling |
| `agents/reference/resource-server.md` | HTTP/OpenAPI integration | Remote operations and API contracts |
| `agents/reference/secrets.md` | Secret handling lifecycle | Secret masking, resolution, storage |
| `agents/reference/metadata.md` | Metadata layering and templates | Metadata merge/render/infer behavior |
| `agents/reference/orchestrator.md` | Orchestration flows | Apply/refresh/diff/list workflows |
| `agents/reference/cli.md` | CLI behavior and output contracts | Command design and UX behavior |
| `agents/reference/e2e.md` | E2E harness and component contracts | E2E profile logic, component onboarding, runtime step orchestration |
| `agents/reference/quality.md` | Quality, testing, and security gates | Validation, test planning, release checks |
| `agents/reference/use-cases.md` | End-to-end examples and edge cases | Scenario design and acceptance tests |

## Request-to-File Load Matrix
| Request Type | Required Files |
|---|---|
| New feature touching orchestration | `agents/reference/interfaces.md`, `agents/reference/domain.md`, `agents/reference/orchestrator.md`, `agents/reference/resource-repo.md`, `agents/reference/resource-server.md`, `agents/reference/metadata.md`, `agents/reference/quality.md` |
| CLI command or output change | `agents/reference/interfaces.md`, `agents/reference/cli.md`, `agents/reference/orchestrator.md`, `agents/reference/domain.md`, `agents/reference/quality.md` |
| Metadata behavior change | `agents/reference/interfaces.md`, `agents/reference/metadata.md`, `agents/reference/domain.md`, `agents/reference/resource-server.md`, `agents/reference/quality.md` |
| Secret behavior change | `agents/reference/interfaces.md`, `agents/reference/secrets.md`, `agents/reference/orchestrator.md`, `agents/reference/quality.md` |
| Context/config change | `agents/reference/interfaces.md`, `agents/reference/context-config.md`, `agents/reference/domain.md`, `agents/reference/quality.md` |
| E2E harness/profile/component change | `agents/reference/interfaces.md`, `agents/reference/e2e.md`, `agents/reference/quality.md`, `agents/reference/use-cases.md` |
| Architecture/refactor proposal | `agents/reference/interfaces.md`, `agents/reference/architecture.md`, `agents/reference/code.md`, `agents/reference/quality.md` |
| Spec authoring only | `agents/reference/interfaces.md`, targeted domain file, `agents/reference/code.md`, `agents/reference/quality.md` |
| Instruction/skill workflow change | `agents/reference/interfaces.md`, `agents/reference/code.md`, `agents/reference/quality.md` plus affected `AGENTS.md`/`agents/skills/*` files |
| Quality strategy/test-policy change | `agents/reference/interfaces.md`, `agents/reference/quality.md`, `agents/reference/use-cases.md` |

## Skill Selection Rules
1. Use `agents/skills/spec-router/SKILL.md` to choose minimal context.
2. Use `agents/skills/spec-writer/SKILL.md` when editing specs or instruction files.
3. Use `agents/skills/quality-gate/SKILL.md` when selecting verification scope for behavior, contract, or security changes.
4. Use `agents/skills/spec-auditor/SKILL.md` when validating consistency and coverage.
5. If multiple skills apply, run in order: `spec-router`, `spec-writer`, `quality-gate`, `spec-auditor`.
6. Use `agents/skills/commit-workflow/SKILL.md` whenever preparing commit guidance that the user will execute once the agent finishes.

## Commit guidance (agents)
- Agents MUST NOT create, amend, or push commits themselves; the user runs `git commit` after the agent finishes preparing the change.
- Agents MAY stage touched files to clarify the final diff, but they must leave the final commit execution to the user and should never stage unrelated modifications.
- Agents SHOULD stay familiar with the allowed Conventional Commit patterns (`<type>(<scope>): <summary>` with types such as `feat`, `fix`, `refactor`, `docs`, `test`, `chore`, `ci`, `perf` and scopes like `cli`, `metadata`, `secrets`, `resource-repo`, `resource-server`, `reconciler`, `config`, `docs`, `tests`, `build`, `deps`), but they MUST NOT offer those messages as recommendations unprompted.
- Each logical change needs a focused justification so that a Conventional Commit can still be composed if the user consents to committing; agents should reserve the actual phrasing until the user explicitly asks or approves the commit.
- Before handing off, agents MUST:
  - run the repo’s standard test command(s) (or at least unit tests when the full suite is impractically slow),
  - scan diffs for secrets or unexpected large/binary files,
  - review the prepared diff (for example via `git diff`/`git diff --staged`) for correctness.
- Agents MUST include in their final response:
  - which files changed,
  - the list of commands executed during the work,
  - any blockers or remaining verification needs that the user should address before committing,
  - and a direct question asking whether the agent should commit the prepared changes now (mirroring the plan-mode prompt before execution).

## Engineering Rules
1. Keep architecture and implementation aligned with senior engineering practices.
2. Keep repository structure legible through bounded contexts and explicit names.
3. Keep one dominant reason to change per file.
4. Avoid unnecessary file proliferation; split only for mixed concerns, unrelated churn, review load, or complexity/size growth.
5. Keep shared/public contracts stable and documented in `agents/reference/interfaces.md`.
6. Preserve business intent and invariants, but do not copy legacy structural anti-patterns.
7. Follow language community conventions for changed files.
8. For Go: use idiomatic package layout (`cmd/*` entrypoints, `internal/*` non-public implementation).
9. For Bash: keep scripts ShellCheck-friendly with robust error handling defaults.
10. Keep context configuration aligned with `agents/reference/context-config.md`.
11. Add or update dependencies only when they are trusted, widely adopted, and actively maintained.
12. When dependencies/imports change, align `go.mod`/`go.sum` and run `go mod tidy`.
13. Use risk-based verification: run the fastest checks that cover changed contracts, then escalate only when required by risk.
14. Final responses should report executed verification commands and any residual risk when checks are skipped or blocked.

## Delivery Protocol
1. After fulfilling a request, stage/prepare the touched files and run the required verification commands, but do not execute `git commit`; instead, summarize the prepared change, report the verification commands, and document any blockers. Then ask the user whether they would like the agent to commit the prepared changes (mirroring the plan-mode consent prompt).
2. If the user agrees and asks for help composing the commit, ensure each resulting Conventional Commit message follows the `Commit guidance (agents)` rules and the `agents/skills/commit-workflow/SKILL.md` checklist before suggesting them.
3. When multiple logical concerns exist and the user asks for multiple commits, propose multiple Conventional Commit messages by numbering each message and describing its individual scope.

## Commit workflow
- Committing is opt-in and MUST go through `agents/skills/commit-workflow/SKILL.md` so that all git-related commands are only executed after the host tool’s default approval/confirmation UI (buttons) explicitly accepts them.
- After a task and its validations complete, if the agent’s work left tracked or untracked changes, it SHOULD invoke the commit-workflow skill so the user sees a brief summary, confirms (yes/no), and approves each proposed git command before it runs.

## Spec Quality Criteria
1. Efficiency: keep workflows and rules minimal; reference canonical sources instead of repeating full rule sets.
2. Assertivity: use `MUST`, `SHOULD`, and `MAY` consistently, with explicit conditions and outcomes.
3. Objectivity: define observable, testable expectations and avoid subjective wording without measurable checks.
4. Redundancy control: when duplicate guidance is discovered, keep one canonical source and replace duplicates with references.

## Completion Checklist
1. Changed behavior is captured in the correct domain files.
2. Interface references match `agents/reference/interfaces.md` exactly.
3. Updated examples include at least one corner case.
4. Quality/security impacts are reflected in `agents/reference/quality.md` when applicable.
5. `AGENTS.md` routing rules and `agents/skills/*` workflows are consistent with each other.
6. Verification scope is documented (commands run, blockers, and residual risks).
7. No unnecessary file fragmentation was introduced.
8. Spec updates satisfy the efficiency/assertivity/objectivity/redundancy criteria above.
