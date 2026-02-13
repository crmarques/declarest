# Code Patterns and Implementation Standards

## Purpose
Define coding standards that produce maintainable, testable, and predictable implementations across modules.

## In Scope
1. Module and file design rules.
2. Error handling and logging patterns.
3. Mutation boundaries and idempotency patterns.
4. Testing expectations at implementation time.

## Out of Scope
1. Language formatter configuration details.
2. Runtime deployment topology.
3. Non-functional SRE platform specifics.

## Normative Rules
1. Architecture and implementation decisions MUST meet senior software engineering best practices.
2. Directory structure MUST remain human-legible from the repository tree.
3. Files MUST have a single scoped responsibility and explicit ownership.
4. Files MUST be sufficiently informative and self-contained so a human can understand module intent by reading file names and localized content.
5. Avoid file proliferation; keep cohesive modules unless split triggers apply.
6. Split files when mixed concerns, unstable churn, review cognitive load, or complexity threshold are reached.
7. New split files MUST be narrowly scoped and named by responsibility.
8. Public contracts MUST be authored in `new-agent-specs/agents/interfaces.md` before implementation use.
9. Code style from legacy disorganization MUST NOT be copied.
10. Functions MUST prefer explicit inputs/outputs over implicit global state.
11. Side effects MUST be isolated behind interface boundaries.
12. Error paths MUST be handled explicitly and tested.
13. Secret material MUST never be logged.

## Data Contracts
Implementation pattern requirements:
1. Define boundary structs/types in domain contracts.
2. Keep adapter-specific payloads at adapter boundaries.
3. Normalize resource payloads before persistence or comparison.
4. Use typed error wrappers aligned with `interfaces.md` taxonomy.

File organization guidance:
1. Keep related contract + validator + mapper logic co-located when change cadence is aligned.
2. Separate orchestration from pure transforms.
3. Keep CLI parse/validation separate from execution side effects.

## Failure Modes
1. Hidden cross-module coupling through shared mutable state.
2. Large god files mixing CLI, reconciliation, and adapter concerns.
3. Unstable tests due to non-deterministic ordering.
4. Error handling that drops root cause context.

## Edge Cases
1. Empty payload vs null payload normalization.
2. Numeric equality across string-number mismatch from external APIs.
3. Partial update behavior when metadata suppresses fields.
4. Retry logic accidentally replays non-idempotent mutation.

## Examples
1. Preferred: pure function `ResolveOperationSpec(resourceInfo, metadata, openAPIHint)` with no side effects.
2. Preferred: `Reconciler.Apply` orchestrates managers and returns typed errors without formatting concerns.
3. Avoid: embedding path parsing and HTTP transport code in the same file as CLI argument parsing.
