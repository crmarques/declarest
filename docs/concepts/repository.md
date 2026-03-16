# Repository

The repository is DeclaREST's desired-state store. It holds resource payload files and metadata files in a deterministic path tree.

## Supported repository backends

### `filesystem`

- local directory only
- no remote Git sync features
- simplest option for local testing and CI jobs

### `git`

- local Git repository managed by DeclaREST
- optional remote configuration for push/refresh/reset flows
- same resource/metadata file layout as `filesystem`

## What DeclaREST stores there

- `resource.json` / `resource.yaml` payloads
- `metadata.json` resource overrides
- `_/metadata.json` collection subtree metadata

## Repository in operator mode

In Operator mode, the Git repository is the desired-state source of truth:

- Admins and CI update repository files through normal Git workflows (often using CLI commands plus pull requests).
- `ResourceRepository` reconciles and fetches the configured branch.
- `SyncPolicy` consumes repository state and reconciles `ManagedServer` real state.

In short: CLI/PRs change desired state in Git; Operator reconciles runtime state from that desired state.

## Metadata baseDir can be separate

By default, metadata lives under the same base directory as repository files.

You can override this with:

```yaml
metadata:
  baseDir: /path/to/metadata
```

This is useful when you want:

- payloads and metadata rules in different repositories
- generated payloads but curated metadata
- environment-specific metadata overlays stored outside the main repo

## Repository command workflows

```bash
declarest repository status
declarest repository clean
declarest repository init
declarest repository refresh
declarest repository push
declarest repository reset
```

Notes:

- `repository push` is only valid for `git` repositories.
- `repository clean` discards local uncommitted Git worktree changes and is a no-op on `filesystem` repositories.
- `repository reset` is destructive to local uncommitted changes.
- `repository status` is the fastest way to confirm local/remote sync state before automation.

## Recommended GitOps loop

1. `declarest repository refresh`
2. `declarest resource diff <path>`
3. `declarest resource apply <path>`
4. Review local files in Git
5. `declarest repository push`

See [Repository Operations workflow](../workflows/repository.md) for examples.
