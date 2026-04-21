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
8. Before final response, run the completion checklist and resolve or surface any blocking unmet items.

## Domain File Catalog
| File | Domain | Load When |
|---|---|---|
| `agents/reference/interfaces.md` | Canonical contracts | Always |
| `agents/reference/architecture.md` | Boundaries and dependency rules | Designing components, refactors |
| `agents/reference/code.md` | Code patterns and implementation standards | Implementing or reviewing code |
| `agents/reference/domain.md` | Vocabulary and invariants | Modeling behavior and data |
| `agents/reference/context-config.md` | Context and config semantics | Context loading, overrides, validation |
| `agents/reference/resource-repo.md` | Resource repository and Git/FS semantics | Storage, sync, path handling |
| `agents/reference/managed-service.md` | HTTP/OpenAPI integration | Remote operations and API contracts |
| `agents/reference/secrets.md` | Secret handling lifecycle | Secret masking, resolution, storage |
| `agents/reference/metadata.md` | Metadata layering and templates | Metadata merge/render/infer behavior |
| `agents/reference/metadata-bundle.md` | Metadata bundle manifest contract | `bundle.yaml` shape, strict decode, compatibility gates |
| `agents/reference/orchestrator.md` | Orchestration flows | Apply/refresh/diff/list workflows |
| `agents/reference/k8s-operator.md` | Kubernetes operator contracts | CRD validation, reconcile loops, webhook refresh flows |
| `agents/reference/cli.md` | CLI behavior and output contracts | Command design and UX behavior |
| `agents/reference/e2e.md` | E2E harness and component contracts | E2E profile logic, component onboarding, runtime step orchestration |
| `agents/reference/commit-instructions.md` | Final handoff subject-line format | Final response and explicit commit-message formatting |
| `agents/reference/quality.md` | Quality, testing, and security gates | Validation, test planning, release checks |
| `agents/reference/use-cases.md` | End-to-end examples and edge cases | Scenario design and acceptance tests |

## Request-to-File Load Matrix
| Request Type | Required Files |
|---|---|
| New feature touching orchestration | `agents/reference/interfaces.md`, `agents/reference/domain.md`, `agents/reference/orchestrator.md`, `agents/reference/resource-repo.md`, `agents/reference/managed-service.md`, `agents/reference/metadata.md`, `agents/reference/quality.md` |
| CLI command or output change | `agents/reference/interfaces.md`, `agents/reference/cli.md`, `agents/reference/orchestrator.md`, `agents/reference/domain.md`, `agents/reference/quality.md` |
| Metadata behavior change | `agents/reference/interfaces.md`, `agents/reference/metadata.md`, `agents/reference/domain.md`, `agents/reference/managed-service.md`, `agents/reference/quality.md` |
| Metadata bundle manifest change | `agents/reference/interfaces.md`, `agents/reference/metadata-bundle.md`, `agents/reference/metadata.md`, `agents/reference/context-config.md`, `agents/reference/quality.md` |
| Secret behavior change | `agents/reference/interfaces.md`, `agents/reference/secrets.md`, `agents/reference/orchestrator.md`, `agents/reference/quality.md` |
| Context/config change | `agents/reference/interfaces.md`, `agents/reference/context-config.md`, `agents/reference/domain.md`, `agents/reference/quality.md` |
| Kubernetes operator controller/CRD/webhook change | `agents/reference/interfaces.md`, `agents/reference/k8s-operator.md`, `agents/reference/architecture.md`, `agents/reference/quality.md` |
| OLM bundle/catalog packaging change | `agents/reference/interfaces.md`, `agents/reference/k8s-operator.md`, `agents/reference/architecture.md`, `agents/reference/quality.md` |
| E2E harness/profile/component change | `agents/reference/interfaces.md`, `agents/reference/e2e.md`, `agents/reference/quality.md`, `agents/reference/use-cases.md` |
| Architecture/refactor proposal | `agents/reference/interfaces.md`, `agents/reference/architecture.md`, `agents/reference/code.md`, `agents/reference/quality.md` |
| Spec authoring only | `agents/reference/interfaces.md`, targeted domain file, `agents/reference/code.md`, `agents/reference/quality.md` |
| CI/release workflow change | `agents/reference/interfaces.md`, `agents/reference/code.md`, `agents/reference/quality.md` |
| Instruction/skill workflow change | `agents/reference/interfaces.md`, `agents/reference/code.md`, `agents/reference/quality.md` plus affected `AGENTS.md`/`agents/skills/*` files, and `agents/reference/commit-instructions.md` when final handoff or commit guidance changes |
| Quality strategy/test-policy change | `agents/reference/interfaces.md`, `agents/reference/quality.md`, `agents/reference/use-cases.md` |

## Skill Selection Rules
1. Use `agents/skills/spec-router/SKILL.md` to choose minimal context.
2. Use `agents/skills/spec-writer/SKILL.md` when editing specs or instruction files.
3. Use `agents/skills/quality-gate/SKILL.md` when selecting verification scope for behavior, contract, or security changes.
4. Use `agents/skills/spec-auditor/SKILL.md` when validating consistency and coverage.
5. If multiple skills apply, run in order: `spec-router`, `spec-writer`, `quality-gate`, `spec-auditor`.
6. Use `agents/skills/commit-workflow/SKILL.md` only when the user explicitly asks for commit guidance or commit creation after the request work is complete.

## Final handoff and commit guidance
- Agents MUST NOT create, amend, stage, or push commits during standard request handoff.
- Agents MUST NOT automatically invoke `agents/skills/commit-workflow/SKILL.md` just because tracked or untracked changes remain after request processing.
- Before standard handoff, agents MUST:
  - apply the `Go-file handoff verification` rules below,
  - scan diffs for secrets or unexpected large/binary files,
  - review the prepared diff (for example via `git diff`) for correctness.
- When the request completes successfully with no remaining blocker, the final response MUST contain only one short subject line that obeys `agents/reference/commit-instructions.md`.
- That standard successful final response MUST NOT append work summaries, changed-file lists, command inventories, verification logs, residual-risk notes, or commit questions.
- When the user explicitly asks for commit guidance or commit creation after the request work is complete, agents MUST use `agents/skills/commit-workflow/SKILL.md`.
- When required verification, required repository/bundle synchronization, or another blocking handoff condition cannot complete, the agent MUST report the blocker directly instead of the standard one-line final response.

## Go-file handoff verification
- When the agent changes at least one `.go` file during a request, the agent MUST run `gofmt -w` on every changed Go file before handoff.
- When the agent changes at least one `.go` file during a request, the agent MUST then run `golangci-lint run`.
- When the agent changes at least one `.go` file during a request, the agent MUST fix every finding reported by `golangci-lint run` before handoff.
- When the agent changes at least one `.go` file during a request, the agent MUST then run `go test -race ./...` (or the deepest feasible subset when full race tests are blocked).
- When the agent changes no `.go` files during a request, the agent MAY skip `gofmt -w`, `golangci-lint run`, and `go test -race ./...`.
- If this verification gate cannot complete when required, or if required `golangci-lint run` findings remain unresolved, the agent MUST treat that as a blocker and report it instead of the standard one-line final response.

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
14. Required verification MUST complete before standard handoff; successful standard final responses SHOULD stay minimal and MUST omit verification detail unless the user explicitly asks for it.
15. Inline or explanatory comments that only restate what the code already expresses MUST NOT be added; updates SHOULD remove such non-functional comments and rely on clear naming, structure, and tests instead, while only keeping compile-time directives or exported-API documentation that cannot be conveyed otherwise.

## Bundle Repository Synchronization
1. Canonical metadata bundle content (metadata trees, openapi specs, `bundle.yaml` manifests) lives in the peer monorepo `../declarest-metadata-bundles/bundles/<component>/` — not in `test/e2e/components/managed-service/<component>/`, which now holds only e2e fixtures (compose, cases, scripts, repo-template).
2. When editing metadata or openapi for a managed-service component that maps to a bundle (for example `keycloak`, `rundeck`, `haproxy`), the agent MUST apply the change under `../declarest-metadata-bundles/bundles/<component>/` (the bundles repo), not under `test/e2e/components/managed-service/<component>/`.
3. When the bundles repo hosts a manifest (`bundles/<component>/bundle.yaml`) that references metadata content, update the manifest simultaneously to keep it consistent with the mirrored metadata tree (for example adjust metadataRoot, openapi hints, compatibleManagedService ranges).
4. If the bundles repo is absent or cannot be edited (for example because it is outside writable roots) and a metadata change is required, treat the missing sync as a blocking handoff condition and report it instead of the standard one-line final response.
5. The e2e harness points at the peer bundles repo via `E2E_METADATA_BUNDLES_ROOT` (default `../declarest-metadata-bundles`); when the operator/CLI publishes bundles to GHCR, the same ref (`<component>:<version>`) stays valid and no e2e component wiring has to change.

## Delivery Protocol
1. After fulfilling a request, run the required verification commands (including the `Go-file handoff verification` commands only when at least one `.go` file changed) and complete any required repository or bundle synchronization work, then stop as soon as those tasks finish.
2. If the request is successful and unblocked, emit only the one-line subject required by `agents/reference/commit-instructions.md`.
3. If the user later asks for commit help, ensure each resulting Conventional Commit message obeys `agents/reference/commit-instructions.md` and follows the `agents/skills/commit-workflow/SKILL.md` checklist before suggesting or proposing commit-related commands.
4. When multiple logical concerns exist and the user asks for multiple commits, propose multiple Conventional Commit messages by numbering each message and describing its individual scope.

## Commit workflow
- Committing is opt-in and MUST go through `agents/skills/commit-workflow/SKILL.md` only after the user explicitly asks for commit help or commit creation.
- Agents MUST NOT invoke the commit-workflow skill automatically during standard request handoff.

## Spec Quality Criteria
1. Efficiency: keep workflows and rules minimal; reference canonical sources instead of repeating full rule sets.
2. Assertivity: use `MUST`, `SHOULD`, and `MAY` consistently, with explicit conditions and outcomes.
3. Objectivity: define observable, testable expectations and avoid subjective wording without measurable checks.
4. Redundancy control: when duplicate guidance is discovered, keep one canonical source and replace duplicates with references.

## Completion Checklist
1. Changed behavior is captured in the correct domain files.
2. Interface references match `agents/reference/interfaces.md` exactly.
3. Machine-readable schemas under `schemas/*.json` are updated when metadata or context contracts change.
4. Updated examples include at least one corner case.
5. Quality/security impacts are reflected in `agents/reference/quality.md` when applicable.
6. `AGENTS.md` routing rules and `agents/skills/*` workflows are consistent with each other.
7. Verification scope is completed before standard handoff, and any blocking gap is surfaced instead of the standard one-line final response.
8. No unnecessary file fragmentation was introduced.
9. Spec updates satisfy the efficiency/assertivity/objectivity/redundancy criteria above.
