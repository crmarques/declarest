# Code Patterns and Implementation Standards

## Purpose
Define Go implementation patterns for side-effect isolation, comment policy, and controller purity that keep behavior predictable and testable.

## Normative Rules
1. Functions MUST take explicit inputs and return explicit outputs rather than read or mutate implicit global state.
2. Side effects (IO, network, clock, filesystem) MUST be isolated behind interfaces declared in their owner packages; pure transforms MUST NOT perform side effects.
3. Boundary types MUST live in their owner packages (`config`, `repository`, `metadata`, `managedservice`, `secrets`, `orchestrator`); provider-specific payloads MUST stay inside provider boundaries (`internal/providers/*`).
4. Dependency wiring MUST be centralized in composition roots (for example `internal/bootstrap`); other packages MUST NOT instantiate their own dependencies.
5. Files MUST separate orchestration from pure transforms, and CLI parsing/validation from execution side effects; a single file MUST NOT mix CLI, orchestration, and provider/transport logic.
6. Kubernetes controller code MUST isolate pure planning/normalization (sync planning, scheduling, classification) from cluster IO (status updates, object fetches, patches) so deterministic behavior is unit-testable without Kubernetes API wiring.
7. Resource payloads MUST be normalized before persistence and before comparison, including empty-object-vs-`null` and string-vs-number representations from external APIs.
8. Inline or explanatory comments that only restate what the code already expresses MUST NOT be added; contributors MUST instead improve naming, structure, or tests, and SHOULD prune existing non-functional comments. Comments MAY remain only for exported-API documentation or compile-time directives that cannot be expressed otherwise.
9. Retry logic MUST NOT replay non-idempotent mutations.
10. Error paths MUST preserve root-cause context per the error taxonomy in agents/reference/interfaces.md; this file requires wrappers to carry actionable context.
11. Secret material MUST never be logged or included in error messages; full secret lifecycle is owned by agents/reference/secrets.md.
12. Contract, validator, and mapper logic for one concept SHOULD be co-located when their change cadence is shared, rather than fragmented across files (this complements the one-dominant-reason-to-change rule in AGENTS.md).

## References
- Go type/interface/method contracts, error taxonomy, determinism, IO expectations: agents/reference/interfaces.md.
- Layer boundaries, allowed/forbidden deps, provider-contract-vs-sibling-instantiation rules: agents/reference/architecture.md.
- Strict context YAML parsing (unknown-key rejection, one-of invariants): agents/reference/context-config.md.
- `gofmt`, `golangci-lint`, idiomatic package/export conventions, global engineering policy: AGENTS.md.

## Examples
1. Preferred: `metadata.ResolveOperationSpec(...)` is pure and returns a value with no side effects, so it is unit-testable directly.
2. Preferred: `orchestrator.Orchestrator.Apply` coordinates managers behind interfaces and returns typed errors that wrap root cause.
3. Avoid: a CLI-parsing file that also contains HTTP transport and filesystem path logic.
4. Avoid: a reconcile method that interleaves sync-plan computation with `Status().Update` calls, blocking unit tests without a live API server.
