# DeclaREST

<p align="center">
    <img src="assets/logo.png" alt="Logo" width="200">
</p>

DeclaREST syncs resources between a Git-backed repository and a target server, letting teams manage its configuration state declaratively (*GitOps*) through its native REST APIs.

## What it solves

- Replace *ad-hoc* API scripts with versioned, reviewable files in Git.
- Detect drift between repository definitions and live systems.
- Promote changes safely across environments using named contexts.
- Keep secrets out of repository files while still templating them.

## How it works

<p align="center">
    <img src="assets/architecture.png" alt="Logo" width="500">
</p>

1. *Resources* live in the repository keeping the same structure used in *managed server* (`/a/b/c`).
2. `declarest resource get --save` pulls a *resource* from the *REST API server* into *Git repository*.
3. `declarest resource apply` pushes *resource* state from repository back to the *API REST server*.
4. `declarest resource diff` shows the state differences for a *resource* between the *Git repository* (*desired state*) and the *managed server* (*actual state*).
5. *Metadata* can define how a *resource* maps to its REST API endpoint (useful when the target API drifts from REST conventions) and can also mark sensitive attributes as *secrets* so they’re stored and managed in a secure *secret store* (outside Git repo).

## When to use DeclaREST

Use DeclaREST when you want a declarative workflow for REST-managed resources and need:

- Deterministic mapping between repo paths and API endpoints.
- Repeatable reconciliation that you can automate or review.
- A clear separation between configuration and secrets.

## Quick links

- [Getting started](getting-started/quickstart.md)
- [Concepts](concepts/overview.md)
- [Configuration reference](reference/configuration.md)
- [Contributing and development](contributing.md)