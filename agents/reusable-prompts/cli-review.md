You are a senior software engineer and product-minded CLI architect. Review this repository's CLI end-to-end and propose improvements to UX, correctness, efficiency, and maintainability. The project is a declarative, Git-backed sync engine for REST-based systems: users define resources and connection contexts in files, then the CLI plans, diffs, and applies changes to remote REST APIs while managing metadata layering, templates, and secrets through a secret-store abstraction.

This is a review and planning task. Do not implement changes automatically.

PRIMARY GOALS
1. UX quality and discoverability
- Ensure the CLI is intuitive for first-time users and efficient for power users.
- Make help output, command naming, global flags, and common workflows consistent and self-explanatory.
- Ensure common read, diff, and apply flows are short and obvious, while advanced features stay discoverable.

2. Correctness and flow integrity
- Validate each command's flow: bootstrap, context selection, metadata resolution, auth, secrets, repository access, HTTP requests, output, and exit codes.
- Ensure error handling is consistent, typed, and actionable.

3. Flag, env, and config consistency
- Audit all global and local flags, their defaults, env-var mappings, config-file support, and precedence.
- Eliminate redundant or overlapping flags and ambiguous naming.
- Ensure the same concept behaves the same way across commands.

4. Efficiency and duplication reduction
- Identify duplicated parsing, validation, metadata rendering, git/repository access, output formatting, command metadata dispatch, and dependency wiring.
- Propose centralization through cohesive helpers or facades rather than ad-hoc per-command logic.

REVIEW METHOD

1. CLI inventory
- Enumerate the full command tree and summarize each leaf command in one line.
- List all global flags in a table with: name, shorthand, type, default, and description.
- For each leaf command, capture:
  - required args,
  - local flags,
  - whether help includes examples,
  - any obvious naming or validation problems.

2. Workflow tracing
- Trace and validate the main user journeys:
  - bootstrap and startup gating,
  - context selection and override resolution,
  - read-only flows (`get`, `list`, `diff`, `describe`, `show`, `resolve`),
  - mutation flows (`save`, `apply`, `create`, `update`, `delete`, repository sync),
  - metadata preview and render flows,
  - raw HTTP request flows,
  - secrets masking, resolving, detecting, and storage,
  - repository clone/init/refresh/status/push/reset flows,
  - output and status footer behavior.
- For each flow, trace the call path from Cobra command to services/managers and confirm the user experience, correctness, and safety properties.

3. Flag, env, and config audit
- For every important flag, evaluate:
  - naming quality and shorthand quality,
  - default and allowed values,
  - validation and mutually exclusive combinations,
  - precedence between CLI flags, env vars, config, and built-in defaults,
  - consistency across commands,
  - whether the flag silently does nothing in any scenario.
- Pay special attention to global flags and secret-revealing flags.

4. Output and scripting stability audit
- Verify:
  - stdout vs stderr separation,
  - text vs JSON vs YAML output consistency,
  - auto-format behavior for terminal vs pipe,
  - exit-code consistency,
  - color and no-color behavior,
  - status footer behavior,
  - secret redaction.

5. Startup, scaffolding, and duplication audit
- Look for:
  - repeated command-tree construction,
  - hardcoded command-path metadata switches,
  - duplicated dependency structs or `Require*` helper stacks,
  - manual pre-parsing that can drift from Cobra,
  - repeated input-flag binding or output-resolution helpers,
  - duplicated validation or output policy logic across command packages.

6. Efficiency and operator experience
- Identify repeated work in a single CLI invocation: config loading, metadata resolution, repository scans, OpenAPI loading, client construction, or output conversion.
- Note missing progress feedback for long-running operations such as git, network calls, auth, or bundle resolution.

7. Bundle follow-up
- If sibling bundle repositories exist (for example `../declarest-bundle-keycloak` or `../declarest-bundle-rundeck`), review them as secondary scope:
  - README usage examples,
  - metadata validation in CI,
  - example resources,
  - documentation for complex transforms,
  - version-range guidance.

HIGH-VALUE ISSUES TO CONFIRM OR REFUTE
- Missing env-var support for global flags.
- Duplicated dependency containers or validation wrappers between CLI and app layers.
- Hardcoded command path matching for bootstrap, output, or status behavior.
- Building the Cobra tree multiple times on startup or completion paths.
- Manual status or color pre-parsing that can diverge from Cobra flag parsing.
- Inconsistent flag names or shorthand meanings across commands.
- Missing or sparse `Example:` help text on leaf commands.
- Fragmented output-format resolution helpers and inconsistent `--output auto` behavior.
- Missing validation for suspicious flag combinations.
- Missing `--dry-run` support on mutation commands.
- Repeated input-flag binding and inconsistent content-type help text.
- Missing CLI preferences config for non-context settings.
- Missing progress indicators for long-running operations.
- Bundle-project documentation and metadata-validation gaps.

DELIVERABLES

A) Current CLI overview
- Command tree.
- Global flags table.
- Primary workflows and user journeys.

B) Findings (prioritized P0/P1/P2)
For each finding include:
- Evidence: file paths plus symbols.
- Problem: UX, correctness, efficiency, duplication, or security.
- Impact: user confusion, wrong behavior, instability, maintenance cost, or safety risk.
- Recommendation: concrete change direction.

C) Flag and config precedence spec (proposed)
- A single explicit precedence order.
- Proposed env-var mapping for important global flags.
- Naming normalization recommendations, including breaking changes if justified.

D) Target UX improvements
- Command naming or structure adjustments, if needed.
- Help-text improvements, examples, and cross-references.
- Output standardization and exit-code policy.
- Safe preview and destructive-operation guardrail improvements.

E) Implementation plan
- Propose a sequence of small, mergeable PR-sized milestones.
- For each step include:
  - scope,
  - files or packages affected,
  - expected benefit,
  - risk level,
  - verification needed.
- Prefer refactors that centralize behavior instead of spreading more helpers across command packages.

OUTPUT RULES
- Be evidence-driven. Do not assume a problem exists unless the code supports it.
- Use exact file paths and symbol names for every substantive finding.
- Keep the recommendations incremental and implementation-ready.
- If you propose breaking changes, explain whether they need a compatibility period or can be applied directly.
- Do not implement changes unless explicitly asked.

SUCCESS CRITERIA
- The review yields a clear CLI inventory, a defensible priority stack of issues, a concrete precedence spec, and a realistic implementation plan.
- Recommendations improve first-run usability, scripting stability, safety, and maintainability without hand-wavy abstractions.
