# Repository and Git Workflows

> This page covers repository lifecycle operations and context management. See [Core Concepts](core-concepts.md) for an overview of the repository model.

## Check repository state

```bash
declarest repository status     # local/remote sync state
declarest repository check      # validate repository config
declarest repository tree       # deterministic directory view
```

Run these before any automated pipeline.

## Initialize

```bash
declarest repository init
```

- **Filesystem**: prepares a local directory.
- **Git**: initializes or syncs the local Git repo. Auto-creates `.git` when the directory exists but has no Git metadata.

## Refresh from remote

```bash
declarest repository refresh
```

Use before editing or applying if other users or automation may have pushed changes.

## Commit and inspect history

```bash
declarest repository commit --message "manual metadata adjustments"
declarest repository history --oneline --max-count 10
```

Use explicit commits when changes were made outside auto-commit flows and you want a commit boundary before push.

## Push to remote

```bash
declarest repository push
```

Only valid for `git` repositories. Verify remote state before force-pushing.

## Reset to remote (destructive)

```bash
declarest repository reset
```

Discards all local uncommitted and unpushed changes. Use only when you intentionally want to start fresh from remote state.

## Clean local worktree (destructive)

```bash
declarest repository clean
```

Removes tracked edits and untracked files without changing committed branch history. No-op on `filesystem` repositories.

## Recommended daily loop

```bash
# 1. Sync with remote
declarest repository refresh
declarest repository status

# 2. Review and reconcile
declarest resource diff /corporations/acme
declarest resource apply /corporations/acme

# 3. Verify and push
declarest repository status
declarest repository history --oneline --max-count 5
declarest repository push
```

## CI/CD patterns

| Backend | Best for |
|---------|----------|
| `filesystem` | Ephemeral CI jobs that only need reconciliation |
| `git` | Jobs that manage promotion branches or push generated updates |

Tips:

- Run `repository status` before destructive or publish steps.
- Use `repository history` to verify expected commits before pushing.
- Use `repository clean` to remove worktree noise before rerunning a workflow.

## Editing contexts

### Inspect current context

```bash
declarest context current       # active context name
declarest context show          # full context view including shared credentials
declarest context resolve       # resolved runtime config
```

### Test overrides without changing stored config

```bash
declarest context resolve \
  --set managedService.http.url=https://staging-api.example.com \
  --set metadata.baseDir=/srv/declarest/staging-metadata
```

Preview runtime overrides before committing to a config change.

### Validate and import config

```bash
# generate template
declarest context print-template > /tmp/contexts.yaml

# edit, then validate
declarest context validate --payload /tmp/contexts.yaml

# import as new context
declarest context add --payload /tmp/contexts.yaml --set-current

# update existing context
declarest context update --payload /tmp/contexts.yaml
```
