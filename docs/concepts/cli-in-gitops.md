# CLI role in GitOps

In DeclaREST GitOps, the CLI is the authoring and validation tool for desired state.

The Operator is the continuous reconciler, but the CLI is still how admins make high-quality changes.

## What admins do with the CLI

Typical flow:

1. Pull/import current state (`resource save`, `repository refresh`).
2. Edit payloads and metadata in Git-tracked files.
3. Validate intent (`resource diff`, `metadata render`, optional apply in lower env).
4. Commit and push changes for review/merge.

After merge, Operator reconciliation updates real state.

## Safe change patterns

- Prefer small, path-scoped commits.
- Always run `resource diff <path>` before push.
- Use metadata compare suppression for noisy fields.
- Use secret-safe flows (`resource save --handle-secrets`).
- Avoid direct production API edits that bypass Git.

## CI usage patterns

Common CI stages:

1. Validate config/context and metadata syntax.
2. Run `resource diff` for changed paths.
3. Optionally run apply in pre-production.
4. Merge to operator-tracked branch.

The Operator then performs continuous apply/prune in-cluster.

## CLI-only mode vs Operator mode

- CLI-only: good for explicit/manual control and one-shot jobs.
- Operator: best for always-on convergence and drift correction.

Most teams use both: CLI for authoring, Operator for runtime reconciliation.
