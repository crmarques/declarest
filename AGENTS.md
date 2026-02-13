# AGENTS

## Purpose
This bootstrap file defines how coding agents operate in this repository rebuild. Follow this file first, then load only the domain files needed for the request.

## Startup Protocol
1. Read `AGENTS.md`.
2. Determine request intent and affected bounded contexts.
3. Load `agents/interfaces.md` first for canonical contracts.
4. Load only the minimal additional domain files from the matrix below.
5. If authoring or revising specs, trigger `spec-writer`.
6. If validating consistency or reviewing specs, trigger `spec-auditor`.
7. Before finalizing, run the completion checklist in `spec-auditor`.

## Domain File Catalog
| File | Domain | Load When |
|---|---|---|
| `agents/interfaces.md` | Canonical contracts | Always |
| `agents/architecture.md` | Boundaries and dependency rules | Designing components, refactors |
| `agents/code.md` | Code patterns and implementation standards | Implementing or reviewing code |
| `agents/domain.md` | Vocabulary and invariants | Modeling behavior and data |
| `agents/context-config.md` | Context and config semantics | Context loading, overrides, validation |
| `agents/resource-repo.md` | Resource repository and Git/FS semantics | Storage, sync, path handling |
| `agents/resource-server.md` | HTTP/OpenAPI integration | Remote operations and API contracts |
| `agents/secrets.md` | Secret handling lifecycle | Secret masking, resolution, storage |
| `agents/metadata.md` | Metadata layering and templates | Metadata merge/render/infer behavior |
| `agents/reconciler.md` | Orchestration and reconciliation flows | Apply/refresh/diff/list workflows |
| `agents/cli.md` | CLI behavior and output contracts | Command design and UX behavior |
| `agents/quality.md` | Quality, testing, and security gates | Validation, test planning, release checks |
| `agents/use-cases.md` | End-to-end examples and edge cases | Scenario design and acceptance tests |

## Request-to-File Load Matrix
| Request Type | Required Files |
|---|---|
| New feature touching orchestration | `interfaces.md`, `domain.md`, `reconciler.md`, `resource-repo.md`, `resource-server.md`, `metadata.md`, `quality.md` |
| CLI command or output change | `interfaces.md`, `cli.md`, `reconciler.md`, `domain.md`, `quality.md` |
| Metadata behavior change | `interfaces.md`, `metadata.md`, `domain.md`, `resource-server.md`, `quality.md` |
| Secret behavior change | `interfaces.md`, `secrets.md`, `reconciler.md`, `quality.md` |
| Context/config change | `interfaces.md`, `context-config.md`, `domain.md`, `quality.md` |
| Architecture/refactor proposal | `interfaces.md`, `architecture.md`, `code.md`, `quality.md` |
| Spec authoring only | `interfaces.md`, targeted domain file, `quality.md` |

## Skill Selection Rules
1. Use `skills/spec-router/SKILL.md` to choose minimal files for context loading.
2. Use `skills/spec-writer/SKILL.md` for creating or updating instruction/spec files.
3. Use `skills/spec-auditor/SKILL.md` for consistency and coverage audits.
4. If multiple skills apply, run them in order: `spec-router`, `spec-writer`, `spec-auditor`.

## Mandatory Engineering Rules
1. Architecture and implementation decisions MUST meet senior software engineering best practices.
2. Directory structure MUST be human-legible from the tree alone through bounded contexts and predictable naming.
3. Files MUST have scoped responsibility and clear ownership.
4. Files MUST be sufficiently informative and self-contained so humans can understand purpose and behavior from structure and content quickly.
5. Agents MUST avoid file proliferation and prefer cohesive files unless splitting is justified.
6. Agents MUST split files when at least one trigger is true: mixed concerns, unstable churn from unrelated edits, growing review cognitive load, or exceeded size/complexity threshold.
7. New files created due to splitting MUST be dedicated and narrowly scoped.
8. Public interfaces and shared types MUST remain stable and centrally documented in `agents/interfaces.md`.
9. Agents MUST not inherit style from legacy disorganization; only business intent, rules, and invariants are retained.
10. Architecture, file structure, and code conventions MUST follow the target language's community best practices.
11. Go changes MUST follow idiomatic Go module/package conventions, including `cmd/*` for executables and `internal/*` for non-public implementation packages.
12. Bash changes (including test scripts) MUST follow community shell standards and be lintable with ShellCheck-friendly patterns and robust error-handling defaults.
13. Context configuration MUST follow the canonical YAML contract documented in `agents/context-config.md`.

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
