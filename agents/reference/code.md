# Code Patterns and Implementation Standards

## Purpose
Define implementation patterns that keep behavior predictable, testable, and maintainable.

## In Scope
1. Module and file design rules.
2. Error handling and side-effect boundaries.
3. Data-shape normalization rules.
4. Implementation-time testing expectations.

## Out of Scope
1. Formatter tool configuration details.
2. Deployment topology.
3. CI vendor specifics.

## Normative Rules
1. Global engineering and organization policies in `AGENTS.md` apply to all code changes.
2. Functions MUST prefer explicit inputs/outputs over implicit global state.
3. Side effects MUST be isolated behind interfaces declared in owner packages.
4. Error paths MUST preserve root cause context and map to canonical error taxonomy.
5. Secret material MUST never be logged or included in error messages.
6. Dependency wiring MUST remain centralized in `core`.
7. Context YAML parsing MUST be strict (unknown keys rejected; one-of invariants enforced).
8. Provider packages MUST implement owner contracts and MUST NOT instantiate sibling providers.
9. Go sources MUST be `gofmt`-formatted and follow idiomatic package/export conventions.

## Data Contracts
Implementation structure:
1. Keep boundary types in owner packages (`config`, `repository`, `metadata`, `server`, `secrets`, `orchestrator`).
2. Keep provider-specific payloads inside provider boundaries (`internal/providers/*`).
3. Normalize resource payloads before persistence and comparison.
4. Use typed error wrappers aligned with `agents/reference/interfaces.md`.

File design guidance:
1. Co-locate contract/validator/mapper logic when change cadence is shared.
2. Separate orchestration from pure transforms.
3. Separate CLI parsing/validation from execution side effects.

## Failure Modes
1. Hidden coupling through shared mutable state.
2. Mixed-concern files that combine CLI, orchestration, and provider logic.
3. Non-deterministic tests due to unstable ordering.
4. Error wrappers that drop actionable context.

## Edge Cases
1. Empty object vs `null` payload normalization.
2. Numeric equality across string/number representations from external APIs.
3. Partial updates with metadata suppression/filter directives.
4. Retry logic replaying non-idempotent mutations.

## Examples
1. Preferred: pure `metadata.ResolveOperationSpec(...)` with no side effects.
2. Preferred: `orchestrator.Orchestrator.Apply` orchestrates managers and returns typed errors.
3. Avoid: CLI parsing file also contains HTTP transport and filesystem path logic.
