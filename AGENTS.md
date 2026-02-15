# AGENTS

## Purpose
This bootstrap file defines how coding agents operate in this repository rebuild. Reference documents live under `agents/reference/`, reusable skills live under `agents/skills/`; load only the domain files needed for the request.

## Startup Protocol
1. Read `AGENTS.md`.
2. Determine request intent and affected bounded contexts.
3. Load `agents/reference/interfaces.md` first for canonical contracts.
4. Load only the minimal additional domain files from the matrix below.
5. If authoring or revising specs, trigger `spec-writer`.
6. If validating consistency or reviewing specs, trigger `spec-auditor`.
7. Before finalizing, run the completion checklist in `spec-auditor`.

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
| `agents/reference/reconciler.md` | Orchestration and reconciliation flows | Apply/refresh/diff/list workflows |
| `agents/reference/cli.md` | CLI behavior and output contracts | Command design and UX behavior |
| `agents/reference/quality.md` | Quality, testing, and security gates | Validation, test planning, release checks |
| `agents/reference/use-cases.md` | End-to-end examples and edge cases | Scenario design and acceptance tests |

## Request-to-File Load Matrix
| Request Type | Required Files |
|---|---|
| New feature touching orchestration | `agents/reference/interfaces.md`, `agents/reference/domain.md`, `agents/reference/reconciler.md`, `agents/reference/resource-repo.md`, `agents/reference/resource-server.md`, `agents/reference/metadata.md`, `agents/reference/quality.md` |
| CLI command or output change | `agents/reference/interfaces.md`, `agents/reference/cli.md`, `agents/reference/reconciler.md`, `agents/reference/domain.md`, `agents/reference/quality.md` |
| Metadata behavior change | `agents/reference/interfaces.md`, `agents/reference/metadata.md`, `agents/reference/domain.md`, `agents/reference/resource-server.md`, `agents/reference/quality.md` |
| Secret behavior change | `agents/reference/interfaces.md`, `agents/reference/secrets.md`, `agents/reference/reconciler.md`, `agents/reference/quality.md` |
| Context/config change | `agents/reference/interfaces.md`, `agents/reference/context-config.md`, `agents/reference/domain.md`, `agents/reference/quality.md` |
| Architecture/refactor proposal | `agents/reference/interfaces.md`, `agents/reference/architecture.md`, `agents/reference/code.md`, `agents/reference/quality.md` |
| Spec authoring only | `agents/reference/interfaces.md`, targeted domain file, `agents/reference/quality.md` |

## Skill Selection Rules
1. Use `agents/skills/spec-router/SKILL.md` to choose minimal files for context loading.
2. Use `agents/skills/spec-writer/SKILL.md` for creating or updating instruction/spec files.
3. Use `agents/skills/spec-auditor/SKILL.md` for consistency and coverage audits.
4. If multiple skills apply, run them in order: `spec-router`, `spec-writer`, `spec-auditor`.

## Mandatory Engineering Rules
1. Architecture and implementation decisions MUST meet senior software engineering best practices.
2. Directory structure MUST be human-legible from the tree alone through bounded contexts and predictable naming.
3. Files MUST have scoped responsibility and clear ownership.
4. Files MUST be sufficiently informative and self-contained so humans can understand purpose and behavior from structure and content quickly.
5. Agents MUST avoid file proliferation and prefer cohesive files unless splitting is justified.
6. Agents MUST split files when at least one trigger is true: mixed concerns, unstable churn from unrelated edits, growing review cognitive load, or exceeded size/complexity threshold.
7. New files created due to splitting MUST be dedicated and narrowly scoped.
8. Public interfaces and shared types MUST remain stable and centrally documented in `agents/reference/interfaces.md`.
9. Agents MUST not inherit style from legacy disorganization; only business intent, rules, and invariants are retained.
10. Architecture, file structure, and code conventions MUST follow the target language's community best practices.
11. Go changes MUST follow idiomatic Go module/package conventions, including `cmd/*` for executables and `internal/*` for non-public implementation packages.
12. Bash changes (including test scripts) MUST follow community shell standards and be lintable with ShellCheck-friendly patterns and robust error-handling defaults.
13. Context configuration MUST follow the canonical YAML contract documented in `agents/reference/context-config.md`.
14. Dependencies introduced on the agent’s behalf MUST originate from trustworthy, widely adopted, well-maintained packages; newly created, unmaintainable, or obscure libraries MUST be avoided.
15. Before adding or updating a dependency, confirm the most recent stable release and target that version, ensuring `go.sum` (and any vendored artifacts) stay in sync.
16. When imports change, verify `go.mod` keeps only explicitly used libraries as direct `require` entries and relegates transitive packages to indirect requirements, running `go mod tidy` or equivalent cleanup to enforce accuracy.
14. Dependencies introduced on the agent’s behalf MUST originate from trustworthy, widely adopted, well-maintained packages; newly created, unmaintainable, or obscure libraries MUST be avoided.
15. Before adding or updating a dependency, confirm the most recent stable release and target that version, ensuring `go.sum` (and any vendored artifacts) stay in sync.
16. When imports change, verify `go.mod` keeps only explicitly used libraries as direct `require` entries and relegates transitive packages to indirect requirements, running `go mod tidy` or equivalent cleanup to enforce accuracy.

## File Organization Policy
1. Keep one dominant reason to change per file.
2. Keep related domain terms and contracts close together.
3. Do not create placeholder files without immediate purpose.
4. Prefer explicit names over generic names.
5. Keep docs concise but complete enough for direct implementation.

## Completion Checklist
1. Confirm changed behavior is captured in the correct domain files.
2. Confirm interface references match `interfaces.md` exactly.
3. Confirm examples include at least one corner case for changed behavior.
4. Confirm quality and security impacts are reflected in `quality.md` when applicable.
5. Confirm no unnecessary file fragmentation was introduced.
