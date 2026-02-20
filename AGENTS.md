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
