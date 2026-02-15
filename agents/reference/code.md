# Code Patterns and Implementation Standards

## Purpose
Define coding standards that produce maintainable, testable, and predictable implementations across modules.

## In Scope
1. Module and file design rules.
2. Error handling and logging patterns.
3. Mutation boundaries and idempotency patterns.
4. Testing expectations at implementation time.

## Out of Scope
1. Formatter configuration details.
2. Runtime deployment topology.
3. SRE platform specifics.

## Normative Rules
1. Architecture and implementation decisions MUST meet senior software engineering best practices.
2. Directory structure MUST remain human-legible from the repository tree.
3. Files MUST have a single scoped responsibility and explicit ownership.
4. Files MUST be sufficiently informative and self-contained so a human can understand module intent by reading file names and localized content.
5. Avoid file proliferation; keep cohesive modules unless split triggers apply.
6. Split files when mixed concerns, unstable churn, review cognitive load, or complexity threshold are reached.
7. New split files MUST be narrowly scoped and named by responsibility.
8. Public contracts MUST be authored in `agents/reference/interfaces.md` before implementation use.
9. Functions MUST prefer explicit inputs/outputs over implicit global state.
10. Side effects MUST be isolated behind interface boundaries.
11. Error paths MUST be handled explicitly and tested.
12. Secret material MUST never be logged.
13. Code structure, naming, and API shape MUST follow the target language community standards for each changed file.
14. Go code MUST follow idiomatic package naming, minimal exported API surface, `cmd/*` entrypoint conventions, and `internal/*` visibility rules for non-public implementation.
15. Bash code used for tests/support MUST use shell community best practices, be ShellCheck-friendly, and apply robust error handling conventions.
16. Dependency assembly MUST be centralized in one composition root package (`core`), not distributed across CLI command handlers or providers.
17. Context YAML parsing MUST use strict decoding that rejects unknown keys and enforces one-of invariants.
18. Provider packages MUST implement owner contracts only and MUST NOT instantiate sibling provider implementations.

## Data Contracts
Implementation pattern requirements:
1. Define boundary structs/types in owner packages (`config`, `repository`, `metadata`, `server`, `secrets`, `reconciler`).
2. Keep provider-specific payloads at provider boundaries (`internal/providers/repository/*`, `internal/providers/server/*`, `internal/providers/secrets/*`).
3. Normalize resource payloads before persistence or comparison.
4. Use typed error wrappers aligned with `interfaces.md` taxonomy.

File organization guidance:
1. Keep related contract + validator + mapper logic co-located when change cadence is aligned.
2. Separate orchestration from pure transforms.
3. Keep CLI parse/validation separate from execution side effects.
4. Keep config loaders, validators, and path resolvers in focused files within the context provider package.

## Failure Modes
1. Hidden cross-module coupling through shared mutable state.
2. Large files mixing CLI, reconciliation, and provider concerns.
3. Unstable tests due to non-deterministic ordering.
4. Error handling that drops root cause context.

## Edge Cases
1. Empty payload vs null payload normalization.
2. Numeric equality across string-number mismatch from external APIs.
3. Partial update behavior when metadata suppresses fields.
4. Retry logic accidentally replays non-idempotent mutation.

## Examples
1. Preferred: pure function `metadata.ResolveOperationSpec(ctx, metadata, operation, payload)` with no side effects.
2. Preferred: `reconciler.ResourceReconciler.Apply` orchestrates managers and returns typed errors without formatting concerns.
3. Avoid: embedding path parsing and HTTP transport code in the same file as CLI argument parsing.
