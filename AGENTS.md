# AGENTS

## Purpose
How coding agents operate in this repository. Canonical specs live in `agents/reference/`; skills in `agents/skills/`; re-runnable review prompts in `agents/reusable-prompts/`.

## Startup Protocol
1. Read this file.
2. Identify the request type and affected bounded contexts.
3. Load `agents/reference/interfaces.md`, then the files from the request-to-file matrix below (union for mixed requests).
4. When authoring or revising any file under `agents/`, run `agents/skills/spec-authoring/SKILL.md`.
5. When the task requires changing code, run `agents/skills/worktree-session/SKILL.md` before editing.
6. When behavior, contracts, or security change, run `agents/skills/quality-gate/SKILL.md` for verification scope.
7. Before the final response, satisfy the Completion Checklist; surface any blocking unmet item instead of the standard handoff.

## Skills
1. `spec-authoring` — route to minimal context, write/revise specs and instruction files, self-audit. Use for any change under `agents/`.
2. `worktree-session` — isolate a coding session in its own branch + git worktree, then rebase onto `main`, fast-forward `main`, and clean up. Use the moment a task requires code changes.
3. `quality-gate` — select and run the smallest verification set that protects the changed contracts.
4. `commit-workflow` — pre-commit handoff for user-approved commits; use ONLY when the user explicitly asks for commit help after the work is complete.

## Reference Files
| File | Owns | Load when |
|---|---|---|
| `interfaces.md` | Types, interfaces, error taxonomy, determinism, IO contracts | Always |
| `architecture.md` | Layer boundaries, dependency rules, orchestrator/app split, OLM packaging boundary | Designing components, refactors |
| `code.md` | Go code patterns and implementation standards | Implementing or reviewing code |
| `domain.md` | Vocabulary and business invariants | Modeling behavior and data |
| `context-config.md` | Context catalog schema, credentials, proxy, env expansion | Context loading, overrides, validation |
| `resource-repo.md` | Repo layout, path safety, Git lifecycle/sync | Storage, sync, path handling |
| `managed-service.md` | HTTP/OpenAPI request construction, auth, throttling | Remote operations and API contracts |
| `secrets.md` | Secret detection, masking, `{{secret .}}` resolution, storage | Secret handling |
| `metadata.md` | Metadata layering, templates, defaults, identity, formats, descendants, inference | Metadata behavior |
| `metadata-bundle.md` | `bundle.yaml` shape, strict decode, compatibility gates, ref forms | Bundle manifest behavior |
| `orchestrator.md` | Apply/refresh/diff/explain/list/request flows, fallbacks | Orchestration workflows |
| `k8s-operator.md` | CRD validation, reconcile loops, webhook refresh, OLM packaging/release | Operator behavior |
| `cli.md` | Command tree, flags, input grammar, output, completion, exit codes | CLI design and UX |
| `e2e.md` | E2E harness, profiles, component contracts, runtime | E2E behavior |
| `commit-instructions.md` | Commit subject-line format | Commit messages |
| `quality.md` | Test strategy and cross-cutting verification gates | Validation, test planning, release checks |
| `use-cases.md` | Cross-domain end-to-end scenarios | Scenario design spanning multiple domains |

## Request-to-File Matrix
| Request type | Required files (plus `interfaces.md` and `quality.md`) |
|---|---|
| Feature touching orchestration | `domain.md`, `orchestrator.md`, `resource-repo.md`, `managed-service.md`, `metadata.md` |
| CLI command or output change | `cli.md`, `orchestrator.md`, `domain.md` |
| Metadata behavior change | `metadata.md`, `domain.md`, `managed-service.md` |
| Metadata bundle manifest change | `metadata-bundle.md`, `metadata.md`, `context-config.md` |
| Secret behavior change | `secrets.md`, `orchestrator.md` |
| Context/config change | `context-config.md`, `domain.md` |
| Operator controller/CRD/webhook change | `k8s-operator.md`, `architecture.md` |
| OLM bundle/catalog packaging change | `k8s-operator.md`, `architecture.md` |
| E2E harness/profile/component change | `e2e.md`, `use-cases.md` |
| Architecture/refactor proposal | `architecture.md`, `code.md` |
| Spec authoring only | targeted domain file, `code.md` |
| CI/release workflow change | `code.md` |
| Instruction/skill workflow change | `code.md`, affected `AGENTS.md`/`agents/skills/*`, and `commit-instructions.md` when handoff/commit guidance changes |
| Quality strategy/test-policy change | `use-cases.md` |

## Canonical Ownership Map
One concept has exactly one owner file; every other file references it instead of restating the rule.
- `interfaces.md`: types, interfaces, method families, error taxonomy, determinism, IO expectations.
- `architecture.md`: layer boundaries, allowed/forbidden dependencies, orchestrator-vs-app split, OLM packaging boundary, interaction flows.
- `code.md`: Go code patterns, side-effect isolation, comment policy, controller purity.
- `domain.md`: vocabulary and business invariants (source-of-truth, identity/alias semantics, `_` namespace, defaults-merge, `format: any`, required-attributes).
- `context-config.md`: context catalog YAML, credentials/`credentialsRef`, proxy model, `${ENV_VAR}` expansion, metadata-source one-of.
- `resource-repo.md`: on-disk layout, path safety, payload discovery, defaults-artifact layout, Git lifecycle/sync/history/tree.
- `managed-service.md`: remote request construction, auth, OpenAPI/Swagger, throttling, media defaults, request-time validation, list-`jq`.
- `secrets.md`: secret lifecycle, `{{secret .}}` key mapping, whole-resource vs attribute secrets, store contracts, redaction.
- `metadata.md`: metadata structure, layering, templates, inference, `resource.defaults`, identity templates, `resource.format`, secret/externalized attributes (declaration), descendant selectors, schema maintenance.
- `metadata-bundle.md`: `bundle.yaml` shape, strict decode, compatibility gates, ref forms, resolver options.
- `orchestrator.md`: orchestration flows, local/remote transitions, bounded fallbacks, runtime defaults merge, binary compare.
- `k8s-operator.md`: CRD spec/validation/status, reconcile, sync planning, webhook receiver, runtime context assembly, OLM packaging/release.
- `cli.md`: command tree, flags, grammar, output contract, completion, exit codes.
- `e2e.md`: harness profiles, component contracts, cases, runtime, handoff.
- `quality.md`: test strategy and cross-cutting verification gates only.

## Engineering Rules
1. Keep architecture and implementation aligned with senior engineering practices and bounded contexts with explicit names.
2. Keep one dominant reason to change per file; split only for mixed concerns, conflict-causing churn, or size growth that impairs safe editing.
3. Keep shared/public contracts stable and documented in `interfaces.md`; refactors affecting public contracts MUST update `interfaces.md` before implementation.
4. Preserve business intent and invariants; do not copy legacy structural anti-patterns.
5. Follow language conventions. For Go: `cmd/*` entrypoints, `internal/*` non-public implementation, `gofmt`-formatted, idiomatic exports. For Bash: ShellCheck-friendly with robust error handling.
6. Add or change dependencies only when trusted, widely adopted, and actively maintained; when imports change, align `go.mod`/`go.sum` and run `go mod tidy`.
7. Do not add comments that restate what code already expresses; rely on naming, structure, and tests. Keep only exported-API docs and compile-time directives. Prune non-functional comments in code you touch.
8. Keep context configuration aligned with `context-config.md`.
9. Use risk-based verification (`quality-gate`): run the fastest checks covering changed contracts, escalate only as risk requires.
10. Spec quality: keep rules minimal and reference canonical sources (efficiency); use `MUST`/`SHOULD`/`MAY` with explicit conditions/outcomes (assertivity); define observable, testable expectations (objectivity); keep one canonical source per rule and replace duplicates with references (redundancy control).

## Changing Code, Verification, and Handoff
1. When a task requires code changes, work through `agents/skills/worktree-session/SKILL.md` (branch + worktree, rebase onto `main`, fast-forward, clean up). That skill is the ONLY sanctioned place where the agent commits and merges autonomously.
2. Outside a worktree session, agents MUST NOT create, amend, stage, or push commits during standard handoff, and MUST NOT auto-invoke `commit-workflow` just because the tree changed. Committing on request goes through `commit-workflow`.
3. Before any handoff, scan diffs for secrets and unexpected large/binary files, review the prepared diff, and complete required verification.
4. On a successful, unblocked request, the final response is a single subject line per `commit-instructions.md` — no summaries, file lists, command inventories, verification logs, or commit questions.
5. When required verification, required bundle synchronization, or another blocking condition cannot complete, report the blocker instead of the standard one-line response.
6. Never `git push` unless the user explicitly asks.

## Go-File Handoff Verification
When at least one `.go` file changed during a request, before handoff the agent MUST: run `gofmt -w` on every changed Go file; run `golangci-lint run` and fix every finding; run `go test -race ./...` (or the deepest feasible subset when full race tests are blocked). When no `.go` files changed, these MAY be skipped. A blocked gate or unresolved finding is a blocker.

## Bundle Repository Synchronization
1. Canonical metadata bundle content (metadata trees, OpenAPI specs, `bundle.yaml`) lives in the peer monorepo `../declarest-metadata-bundles/bundles/<component>/`, not in `test/e2e/components/managed-service/<component>/` (which holds only e2e fixtures).
2. When editing metadata or OpenAPI for a managed-service component mapped to a bundle (e.g. `keycloak`, `rundeck`, `haproxy`), apply the change under the bundles repo, and update that component's `bundle.yaml` simultaneously (metadataRoot, openapi hints, compatibleManagedService ranges) to stay consistent.
3. If the bundles repo is absent or outside writable roots and a metadata change is required, treat the missing sync as a blocking handoff condition and report it.
4. `--metadata-source bundle` (default) resolves `METADATA_BUNDLE_REF` OCI references at runtime via `bundlemetadata.ResolveBundle` (no external `oras` CLI); `--metadata-source dir` resolves from the peer bundles repo at `E2E_METADATA_BUNDLES_ROOT` (default `../declarest-metadata-bundles`) so at least one e2e path exercises disk-sourced bundles.

## Completion Checklist
1. Changed behavior is captured in the correct owner file (per the ownership map).
2. Interface references match `interfaces.md` exactly.
3. `schemas/*.json` are updated when metadata or context contracts change.
4. Updated examples include at least one corner case.
5. Quality/security impacts are reflected per `quality.md` when applicable.
6. The `AGENTS.md` matrix and `agents/skills/*` workflows are mutually consistent.
7. Required verification completed before handoff; any blocking gap is surfaced instead of the standard one-line response.
8. No unnecessary file fragmentation or duplicated rules were introduced.
