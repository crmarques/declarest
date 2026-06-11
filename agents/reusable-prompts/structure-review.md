# Structure Review (architecture + code organization)

Re-runnable review of DeclaREST's architecture, boundaries, domain modeling, package/file organization, interfaces, and naming. Review and plan only — do not implement unless explicitly asked. No backward-compatibility is wanted; favor the leanest design.

Treat `agents/reference/architecture.md`, `code.md`, `domain.md`, and `interfaces.md` as the canonical intent. Flag where the code diverges from them, and where those specs are themselves wrong or stale.

## Method
1. **Map** top-level packages and entrypoints (`cmd/*`, `internal/cli`, `internal/app`, `internal/orchestrator`, `internal/operator`, `internal/providers`, `internal/bootstrap`, domain packages, `api/v1alpha1`). One line per package; a text diagram of the key flows and dependency directions.
2. **Boundaries** — verify dependencies point inward toward domain contracts: CLI/operator are adapters that delegate to `internal/app`/orchestrator and never import providers; only `internal/bootstrap` wires concrete providers; orchestrator holds pure domain workflow, app holds side-effecting UX. List every violation with file + symbol.
3. **Domain modeling** — are entities, identity/alias semantics, and invariants represented explicitly and named in domain vocabulary? Flag infrastructure leakage into domain packages.
4. **Cohesion & distribution** — each package/file has one dominant responsibility; related types+behavior live together; flag fragmentation, overloaded files, and `util/common/helpers` dumping grounds. Recommend split / join / move / rename / remove.
5. **Centralization** — cross-cutting work (metadata resolution, config, secrets, repository/git access, templating, output) goes through its owning manager/facade, not re-implemented at call sites. Flag bypasses (e.g., direct metadata file reads instead of the metadata service).
6. **Interfaces** — for each, record purpose, implementers, consumers, and whether every method is used. Flag god/anemic interfaces, generic names (`Manager`/`Service`/`Provider`/`Handler` without qualification), interfaces with a single implementation and no credible variation, and "interface-by-principle". Interfaces earn their place only at real boundaries (adapters, external integrations, multiple implementations, stable contracts).
7. **Duplication & dead code** — near-duplicate logic, redundant wrappers, unused types/functions/branches, and non-functional comments (code should be self-explanatory). Propose consolidation into the owning domain package.
8. **Naming** — packages/files/types/functions explain intent at first glance and use domain vocabulary consistently.

## Deliverables
- **A. Overview** — structure + flows + dependency directions (text diagram).
- **B. Findings** — prioritized P0 (coupling/boundary blockers) / P1 (structural clarity) / P2 (polish), each with file+symbol evidence, why it matters, and the direction (split/join/move/rename/remove/centralize).
- **C. Target structure** — proposed package layout, responsibilities, and import/boundary rules; interfaces only where they add value.
- **D. Incremental plan** — small, independently mergeable, testable steps with stop points where the design is already improved.

Be evidence-driven (cite real paths/symbols); prefer simple readable solutions over heavy patterns; state assumptions when context is missing.
