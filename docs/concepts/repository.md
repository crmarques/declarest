# Repository

The repository is DeclaREST's desired-state store.
It holds resource payload files and metadata files in a deterministic path tree.

## Supported repository backends

### `filesystem`

- local directory only
- no remote Git sync features
- simplest option for local testing and CI jobs

### `git`

- local Git repository managed by DeclaREST
- optional remote configuration for push/refresh/reset flows
- keeps the same resource/metadata file layout as `filesystem`

## What DeclaREST stores there

- `resource.json` / `resource.yaml` payloads
- `metadata.json` resource overrides
- `_/metadata.json` collection subtree metadata

## Metadata base-dir can be separate

By default, metadata lives under the same base directory as repository files.

You can override this with:

```yaml
metadata:
  base-dir: /path/to/metadata
```

This is useful when you want:

- application payloads and metadata rules in different repositories
- generated payloads but manually curated metadata
- environment-specific metadata overlays stored outside the main repo

## Repository command workflows

```bash
declarest repo status
declarest repo init
declarest repo refresh
declarest repo push
declarest repo reset
```

Notes:

- `repo push` is only valid for `git` repositories.
- `repo reset` is destructive to local uncommitted changes.
- `repo status` is the fastest way to confirm local/remote sync state before automation.

## Recommended GitOps loop

1. `declarest repo refresh`
2. `declarest resource diff <path>`
3. `declarest resource apply <path>`
4. Review local files in Git
5. `declarest repo push`

See [Repository Operations workflow](../workflows/repository.md) for examples.
