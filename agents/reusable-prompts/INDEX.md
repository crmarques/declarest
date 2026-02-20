# Reusable Prompt Index

## Purpose
Provide one discoverable index for reusable review prompts and their intended objective.

## Usage
1. Select the prompt that matches the immediate review goal.
2. Use the prompt as-is or add narrow scope constraints (paths, packages, or command groups).
3. Keep the review non-mutating unless implementation is explicitly requested.

## Prompt Catalog
| File | Objective | Best For |
|---|---|---|
| `agents/reusable-prompts/architecture-review.md` | Evaluate architecture boundaries, domain modeling, and dependency direction. | Package/layout refactors and boundary audits. |
| `agents/reusable-prompts/cli-review.md` | Evaluate CLI UX, flow correctness, and flag/config consistency. | Command/flag/output contract reviews. |
| `agents/reusable-prompts/code-quality-review.md` | Evaluate responsibility distribution, cohesion, duplication, and centralization opportunities. | Repository-wide refactoring plans. |
| `agents/reusable-prompts/efficiency-security-review.md` | Evaluate security, correctness, and performance risk surfaces. | Defensive audits and hardening plans. |
| `agents/reusable-prompts/testing-review.md` | Evaluate test architecture and spec-coverage strategy. | Test-gap analysis and testing-roadmap design. |

## Maintenance Rules
1. Prompt filenames MUST be correctly spelled and stable.
2. Every prompt added to this directory MUST be listed in this index.
3. Prompt objectives SHOULD be one sentence and testable as review outcomes.
