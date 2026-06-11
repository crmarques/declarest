# CLI Review

Re-runnable review of DeclaREST's CLI for UX, correctness, flag/config consistency, and efficiency. DeclaREST is a declarative Git-backed sync engine for REST APIs: users define resources and contexts in files, then the CLI plans, diffs, and applies changes to remote APIs while managing metadata layering, templates, and secrets. Review and plan only — do not implement unless explicitly asked.

`agents/reference/cli.md` is the canonical CLI contract; `interfaces.md` and `orchestrator.md` define the use cases commands map to. Flag where the CLI code diverges from these, and where the spec is stale.

## Method
1. **Inventory** — enumerate the full command tree (one line per leaf) and a global-flag table (name, shorthand, type, default, env var, description). Per leaf: required args, local flags, whether help has an example, naming/validation problems.
2. **Workflow tracing** — trace each main journey from the Cobra command to services/orchestrator and confirm UX, correctness, and safety: startup/context gating; context selection + override resolution; read flows (`get`/`list`/`diff`/`explain`/`metadata`); mutation flows (`save`/`apply`/`create`/`update`/`delete`, repository sync); raw `resource request`; secret mask/resolve/detect; repository init/refresh/status/commit/push; output + status-footer behavior.
3. **Flag/env/config audit** — naming and shorthand quality, defaults and allowed values, validation and mutually-exclusive combinations, precedence (`flag > env > built-in default`), cross-command consistency for the same concept, and flags that silently do nothing. Scrutinize global and secret-revealing flags.
4. **Output & scripting stability** — stdout vs stderr separation, `auto|text|json|yaml` consistency, auto-format for terminal vs pipe, exit-code mapping by error category, `--no-color`/`NO_COLOR`, status footer, secret redaction.
5. **Scaffolding & duplication** — repeated command-tree construction, hardcoded command-path metadata switches, duplicated dependency/`Require*` stacks, manual pre-parsing that can drift from Cobra, repeated input-flag binding or output-resolution helpers. Prefer centralization over more per-command helpers.
6. **Efficiency & operator UX** — repeated work in one invocation (config load, metadata resolution, repository scans, OpenAPI/bundle load, client construction); missing progress feedback for long git/network/auth/bundle operations.

## Confirm or refute (common issues)
Missing env-var support for global flags; duplicated dependency containers between CLI and app layers; hardcoded command-path matching for bootstrap/output/status; building the Cobra tree multiple times on startup/completion; manual status/color pre-parsing that diverges from Cobra; inconsistent flag names/shorthands across commands; sparse `Example:` help; fragmented output-format resolution; missing validation for suspicious flag combinations; missing progress indicators.

## Deliverables
- **A. CLI overview** — command tree, global-flag table, primary journeys.
- **B. Findings (P0/P1/P2)** — each with file+symbol evidence, the problem (UX/correctness/efficiency/duplication/security), impact, and a concrete direction.
- **C. Flag/config precedence spec** — one explicit precedence order, proposed env-var mappings, naming normalization (note breaking changes).
- **D. Target UX** — command-structure/help/example improvements, output + exit-code policy, destructive-operation guardrails.
- **E. Plan** — small mergeable milestones with scope, files, benefit, risk, verification.

Be evidence-driven (exact paths/symbols); keep recommendations incremental and implementation-ready.
