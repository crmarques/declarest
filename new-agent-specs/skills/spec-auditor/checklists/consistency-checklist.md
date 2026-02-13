# Consistency Checklist

## Contract Integrity
1. Every domain file references canonical interfaces from `new-agent-specs/agents/interfaces.md`.
2. No file redefines interface names with conflicting semantics.
3. Error categories align with taxonomy in `interfaces.md`.

## Boundary Integrity
1. `architecture.md` enforces dependency direction and reconciler orchestration boundaries.
2. `code.md` implementation rules do not contradict `architecture.md`.
3. Adapter-specific concerns stay outside domain contracts.

## Behavior Integrity
1. `metadata.md` precedence rules are deterministic and testable.
2. `resource-repo.md` path safety rules reject traversal.
3. `resource-server.md` OpenAPI fallback rules preserve explicit metadata overrides.
4. `secrets.md` non-disclosure rules are strict and enforceable.
5. `reconciler.md` defines idempotent apply behavior and bounded fallbacks.

## CLI and UX Integrity
1. `cli.md` command semantics map to reconciler use cases.
2. Validation and destructive-operation safeguards are explicit.
3. Structured output expectations are stable.

## Quality Coverage Integrity
1. `quality.md` includes required scenario coverage for all high-risk behaviors.
2. Every new normative rule has a corresponding test expectation.
3. Security-sensitive scenarios include negative tests.

## File Organization Integrity
1. Files are scoped by single dominant responsibility.
2. Split triggers are documented and applied consistently.
3. No unnecessary placeholder or duplicate files exist.
