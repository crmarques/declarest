# Consistency Checklist

Mark each item pass / fail / needs-clarification. Give concrete remediation for every fail.

## Contracts
1. Every domain file references canonical interfaces from `agents/reference/interfaces.md`; none redefines an interface with conflicting semantics.
2. Error categories align with the taxonomy in `interfaces.md`.

## Boundaries
1. `architecture.md` dependency directions and the orchestrator-vs-app split are consistent with `code.md` and the domain files.
2. Adapter/provider concerns stay out of domain contracts.

## Behavior
1. `metadata.md` precedence/layering rules are deterministic and testable.
2. `resource-repo.md` path-safety rules reject traversal.
3. `managed-service.md` OpenAPI fallback preserves explicit metadata overrides.
4. `secrets.md` non-disclosure rules are strict and enforceable.
5. `orchestrator.md` apply is idempotent with bounded fallbacks.
6. `k8s-operator.md` reconcile/status/webhook contracts are deterministic and consistent with the CRD/controller behavior.

## CLI / UX
1. `cli.md` commands map to orchestrator use cases; validation and destructive-operation safeguards are explicit; structured output is stable.

## Routing & Skills
1. The `AGENTS.md` request-to-file matrix is canonical and consistent with each `agents/skills/*/SKILL.md`.
2. All skill and file references point to existing paths.

## Quality Coverage
1. Every new normative rule has a traceable test expectation at the lowest effective layer.
2. Security-sensitive and destructive scenarios have negative tests.
3. Verification guidance stays risk-based and proportional.

## Organization & Quality
1. Each file has a single dominant responsibility; no duplicate or placeholder files.
2. Rules do not duplicate canonical guidance (one owner + cross-references).
3. Normative language uses `MUST`/`SHOULD`/`MAY` consistently; requirements are objective and testable.
4. Lists are cleanly numbered with no duplicates.
