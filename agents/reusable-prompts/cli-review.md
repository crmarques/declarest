You are a senior software engineer / product-minded CLI architect. Your task is to review this repository’s CLI end-to-end and propose improvements to UX, correctness, and efficiency. The project is a declarative, Git-backed sync engine for REST-based systems: users define resources and connection contexts in files, then the CLI plans/diffs/applies changes to remote REST APIs while managing metadata layering/templates and secrets through a secret-store abstraction.

Your review must be evidence-driven: inspect the code, enumerate commands/flags, trace execution flows, and find duplication or inconsistent behavior. Provide a concrete improvement plan (step-by-step PR-sized milestones). Do not implement changes automatically.

PRIMARY GOALS
1) UX quality and discoverability
- Ensure the CLI is intuitive for first-time users and efficient for power users.
- Make help output, command naming, and workflows consistent and self-explanatory.
- Ensure “happy path” flows are short and obvious; advanced features are discoverable.

2) Correctness & flow integrity
- Validate each command’s flow is correct: config loading, context selection, auth/secrets, git operations, API calls, outputs, and exit codes.
- Ensure error handling is consistent and actionable.

3) Flag and config consistency
- Audit all flags (global and per-command), their defaults, env-var mappings, and config-file precedence.
- Eliminate redundant/overlapping flags and ambiguous naming.
- Ensure stable behavior: same inputs → same outputs.

4) Efficiency and duplication reduction
- Identify duplicated parsing, validation, metadata rendering, git IO, REST client creation, or output formatting across commands.
- Propose centralization via cohesive managers/facades (e.g., Metadata, Config/Context, Secrets, ResourceRepo/Git, ResourceServer/API client, Output/Formatter).

REVIEW METHOD (do this in order)

1) CLI inventory (what exists today)
- List all commands/subcommands (tree view) and their intended purpose in 1 line each.
- List all global flags and shared behaviors (verbosity, config path, context, output format, no-color, etc.).
- For each command, list:
  - required args,
  - flags,
  - examples shown in help (and note missing ones).

2) Workflows and user journeys (must be coherent)
Identify and validate the key user flows that match the project’s nature:
- Bootstrapping: init/setup, generating skeleton configs, validating config.
- Read-only flows: inspect/show/list/status/export/pull/preview.
- Change flows: plan/diff/apply/sync/push with idempotency guarantees.
- Multi-context / multi-environment: selecting a context, overriding endpoints, switching credentials.
- Metadata layering/templates: how users preview/render and how the engine resolves layers.
- Secrets: how credentials are referenced, loaded, and never leaked to output/logs.
- Git-backed resource repository: clone/init, branch/mirror/workdir patterns, safety checks, and remote operations.
For each flow:
- Trace the call path in code (from Cobra command → services/managers → git/fs/http).
- Confirm the flow is correct, efficient, and provides good UX (messages, progress, errors).

3) Flag audit (deep, systematic)
For EVERY flag:
- Name quality: intent-revealing, consistent style, no vague names.
- Behavior: what it changes, default, whether it’s global or local.
- Precedence rules:
  - flags vs config file vs env vars vs defaults (document and enforce consistently).
- Validation:
  - required combos, mutually exclusive flags, allowed values, empty/zero behavior.
- Consistency:
  - same flag name does the same thing across commands.
- Hidden traps:
  - flags that silently do nothing, ambiguous flags, multiple flags controlling one concept.

4) Output UX and scripting stability
- Ensure outputs are:
  - human-friendly by default (clear summaries, next actions),
  - script-friendly when requested (JSON/YAML output modes),
  - stable over time (avoid ad-hoc formatting changes without versioning).
- Audit:
  - exit codes per scenario (success/no-op/partial failure/validation error/auth error).
  - stdout vs stderr usage (machine output on stdout, diagnostics on stderr).
  - log levels and verbosity controls.
  - color/no-color support and terminal detection.
- Ensure no secrets are printed (tokens, passwords, private keys, auth headers).

5) Performance/efficiency review in CLI context
- Look for repeated work per command (re-loading config, re-parsing metadata, re-building clients, repeated git scans).
- Identify opportunities for:
  - caching within a single run,
  - lazy loading (only load what the command needs),
  - reducing filesystem and git operations,
  - batching HTTP calls where safe,
  - concurrency controls (but deterministic results).
- Confirm timeouts and retries are sane (especially around remote REST APIs and git remotes).

6) Duplication and architecture improvement suggestions
- Identify repeated patterns in commands (same validation, same context resolution, same manager wiring).
- Propose refactors:
  - shared “command scaffolding” functions,
  - unified managers/facades with clear responsibilities,
  - consistent error and output formatting helpers,
  - reducing the number of ad-hoc helper functions spread across CLI packages.

DELIVERABLES (required output format)

A) Current CLI overview
- Command tree
- Global flags
- Primary workflows (user journeys)

B) Findings (prioritized P0/P1/P2)
For each finding include:
- Evidence: file paths + symbols (commands/flags/functions)
- Problem: UX/correctness/efficiency/duplication
- Impact: user confusion, incorrect behavior, instability, maintenance cost
- Recommendation: concrete change direction

C) Flag & config precedence spec (proposed)
- A single, explicit precedence order and rules.
- Proposed naming normalization and deprecations (with migration strategy).

D) Target UX improvements
- Proposed command naming/structure adjustments (if needed).
- Help text improvements (examples, parameter descriptions, gotchas).
- Output mode standardization (text vs JSON/YAML), exit code policy.

E) Implementation plan (step-by-step PRs)
- Small, safe milestones:
  - refactor scaffolding,
  - unify managers/facades,
  - normalize flags,
  - improve help/examples,
  - improve output consistency,
  - add CLI tests (unit + integration style for Cobra commands; golden tests for output where appropriate).

QUALITY BAR / GUARDRAILS
- Keep changes aligned with a declarative GitOps-style workflow:
  - commands should map to conceptual actions (validate/plan/diff/apply/sync/status/export).
- Prefer explicitness over magic:
  - no surprising implicit context switches or hidden defaults.
- Backward compatibility:
  - if you propose breaking changes, include a clear deprecation path and a compatibility period plan.
- Security:
  - never print secrets; sanitize errors; safe defaults for remote operations.

SUCCESS CRITERIA
- A CLI that is easy to understand at first glance, consistent across commands, and efficient.
- Clear, enforceable rules for flags/config/env precedence.
- Reduced duplication via cohesive managers and shared scaffolding.
- Correct, reliable flows for Git-backed declarative sync to REST APIs, including metadata layering and secrets handling.
