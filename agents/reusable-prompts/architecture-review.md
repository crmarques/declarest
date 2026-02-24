You are a senior software engineer / software architect. Your task is to review this Git repository’s architecture end-to-end and propose improvements. Focus on architecture, boundaries, domain modeling, naming, and component interactions. Ignore the internal implementation of functions for now—evaluate design, structure, and contracts.

GOALS
- Assess whether the architecture is clear, cohesive, scalable, and maintainable.
- Evaluate whether the domain design is strong or should be improved (bounded contexts, invariants, vocabulary, boundaries).
- Ensure packages/directories reflect cohesive domains and responsibilities (no “misc dumping grounds”).
- Verify that files, types, interfaces, functions, and their interactions are easy to understand at first glance.
- Reduce accidental complexity: remove unnecessary abstractions, improve naming, clarify boundaries, and simplify dependencies.
- Provide a concrete, incremental improvement plan to make the architecture lean and evolution-friendly.

SCOPE / NON-GOALS
- Do NOT judge business logic correctness or micro-optimizations.
- Do NOT refactor internal function bodies except when strictly necessary to clarify boundaries or fix contract issues.
- Focus on: directory/package structure, public APIs, interfaces, domain boundaries, naming, and dependency direction.

HOW TO REVIEW (step-by-step)

1) Map the current architecture (structure + flows)
- List the main modules/packages and their responsibilities.
- Identify the main user-facing entrypoints (CLI, HTTP server, jobs, library API).
- Describe the key flows (who calls whom) and draw a simple text diagram.
- Identify dependencies between packages and their direction (ideal: dependencies point inward toward the domain/core).

2) Evaluate domain design and boundaries
- Identify the core domain(s) and supporting domains (or bounded contexts).
- Check if domain concepts are represented explicitly (entities, value objects, aggregates, domain services, domain events if applicable).
- Validate domain vocabulary consistency: types and functions should reflect domain terms, not generic patterns.
- Look for leakage of infrastructure concerns into domain packages (e.g., DB/HTTP types, framework glue).
- Identify missing or unclear boundaries and propose better separation.

3) Check package/directory cohesion and containment
- Ensure each package has a single, clear purpose and cohesive content.
- Flag “utility”, “common”, “helpers”, “misc” patterns unless they are well-justified and tightly scoped.
- Verify that related concepts live together (types + behaviors + interfaces near usage).
- Detect cycles, cross-domain imports, and unclear layering.

4) Inventory and audit interfaces (interface-first where it adds value)
For EACH interface, record:
- Location (file/path) and name.
- Purpose/responsibility in 1 sentence (domain-aligned).
- Who consumes it (callers) and who implements it (implementations).
- Methods (signatures) and whether each method is used by consumers.
- Red flags:
  - Too large / low cohesion (“god interface”).
  - Too generic / anemic naming (Manager/Service/Provider/Handler without clear meaning).
  - Duplication (similar interfaces in multiple places).
  - Defined far from the consumer without a reason.
  - Created “by principle” rather than by need.
  - Interface used to hide a concrete type unnecessarily.
  - Mocking complexity indicating a weak boundary.

Decide when interfaces are truly needed:
- Interfaces add value at boundaries: infrastructure adapters, external integrations, plugin points, multiple real implementations, and stable contracts.
- Prefer concrete types when:
  - There is only one real implementation and no credible near-term variation.
  - The interface exists only for “testability” but harms clarity.
Principles:
- “Interfaces belong to the consumer” when appropriate.
- ISP/SRP: small and focused interfaces.
- Avoid “interfaces everywhere”: abstract only where it improves design.

5) Naming and readability audit (first-look comprehensibility)
- Review packages, files, types, interfaces, functions, and method names:
  - Do they explain intent without needing to read implementations?
  - Do they use domain vocabulary consistently?
  - Do names describe responsibility and scope?
- Avoid/refactor vague names:
  - “Options”, “Manager”, “Service”, “Provider”, “Util”, “Common”, “Handler” unless made specific and meaningful.
- Ensure conventions are consistent (e.g., suffixes like *Repository, *Store, *Client, *Adapter only when accurate).

6) Interactions and contracts
- Validate interactions between components:
  - Clear ownership and dependency direction.
  - Minimal knowledge across boundaries (no leaky abstractions).
  - Consistent error handling strategy in public APIs (types/errors).
  - Stable contracts for core domain APIs.
- Confirm that boundaries are enforced by package structure and import rules (not just by comments).

7) Propose refactorings: split / join / remove / rename
For each recommendation, provide:
- What to change (merge/split/remove/rename/move packages).
- Why (signals observed, tied to real code structure/usage).
- Trade-offs (what improves vs potential costs).
- Impact (files/symbols affected, risk, effort).
- Incremental steps (safe refactoring order).
- If helpful: target package layout and target interface/type signatures.

OUTPUT FORMAT (required)

A) Architecture overview (short and factual)
- Current high-level structure
- Key flows and dependency direction (with a simple text diagram)

B) Domain and package assessment
- Identified domains/bounded contexts
- What’s good
- What’s unclear/leaky
- Concrete improvements to domain modeling and boundaries

C) Interface Audit (one entry per interface)
- Findings, red flags, and keep/simplify/remove decisions

D) Naming and readability issues (prioritized)
- Examples with paths/symbols
- Recommended renames and rationale

E) Prioritized issues (P0/P1/P2) with evidence
- P0: architectural blockers / high-risk coupling
- P1: structural clarity / maintainability issues
- P2: polish / consistency improvements

F) Improvement plan (incremental, small PRs)
- Step-by-step plan to reach a lean, understandable architecture
- Each step should be safe, independently mergeable, and testable
- Include “stop points” where the architecture is already improved even if later steps aren’t done

G) Target architecture (proposed end state)
- Package structure proposal
- Boundary rules (who may import whom)
- Interfaces only where they add value
- Simple text diagram of component interactions

H) Validation checklist (final)
- Cohesion, coupling, dependency direction
- Domain purity (infra leakage)
- Interface necessity and size
- Naming clarity (first-look readability)
- Scalability of evolution (new features/extensions)

RULES / GUARDRAILS
- Be skeptical: don’t assume intent—validate against real code structure and usage.
- Avoid cosmetic changes without clear architectural benefit.
- Prefer simple, readable solutions over heavy patterns.
- If context is missing, state assumptions and offer alternatives.
- Do not apply changes automatically; propose only (unless I explicitly ask you to implement).

SUCCESS CRITERIA
- Clearer domain boundaries and package containment.
- Fewer gratuitous interfaces and abstractions; higher cohesion.
- Names that explain themselves at first glance.
- Stable contracts and non-leaky boundaries.
- A practical plan that makes future evolution efficient and scalable.
