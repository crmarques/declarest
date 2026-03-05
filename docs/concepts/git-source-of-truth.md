# Git repository as source of truth

In DeclaREST GitOps, the repository is the canonical desired state.

- Desired state is what is declared in repository files.
- Real state is what the API currently returns.
- Reconciliation moves real state toward desired state.

## Typical repository layout

Example for `/customers/acme`:

```text
customers/
  acme/
    resource.json
    metadata.json        # optional resource-level override
  _/
    metadata.json        # optional collection/subtree defaults
```

The same layout works for CLI and Operator workflows.

## Pull request flow

A common workflow:

1. Import or edit desired state (`resource save`, manual edit, metadata updates).
2. Run `resource diff` and optional validation checks.
3. Open a PR for review.
4. Merge to the tracked branch.
5. Operator reconciles merged state to Managed Server.

## Environment patterns

Two practical patterns are common.

### Branch-based environments

- `main` for production
- `staging` for staging
- separate `ResourceRepository` per branch

Use this when branch protections and promotion rules are central.

### Directory-based environments

```text
envs/
  dev/
  staging/
  prod/
```

Each `SyncPolicy.source.path` points at one environment directory. Use this when you want one branch with clear path-level ownership.

## Guardrails

- Keep logical paths stable; avoid frequent rename churn.
- Treat direct API edits as drift unless intentionally reconciled back to Git.
- Prefer PR review for production changes.
- Use small commits scoped to one logical area.
