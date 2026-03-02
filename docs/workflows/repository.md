# Repository Operations

This page covers repository lifecycle commands and how to use them safely in local and CI workflows.

## Check repository state first

```bash
declarest repository status
declarest repository check
declarest repository tree
```

Use this before any automated apply/push pipeline.
`repository tree` gives a quick deterministic directory-only view of the local repository structure.

## Initialize the repository

```bash
declarest repository init
```

Behavior depends on context configuration:

- `filesystem`: prepares/uses a local directory
- `git`: initializes or syncs the local Git repo according to config

Git-backed commands also auto-initialize `.git` when the configured repository exists but has no Git metadata yet.

## Refresh local from remote (git repos)

```bash
declarest repository refresh
```

Use this before editing/applying if other automation or users might have pushed changes.

## Push local changes (git repos)

```bash
declarest repository push
```

If you intentionally need a non-fast-forward push, use the explicit force-push option exposed by the CLI and verify the remote state first.

## Commit and inspect local history (git repos)

```bash
declarest repository commit --message "manual metadata adjustments"
declarest repository history --oneline --max-count 10
```

Use this when repository changes were made outside auto-commit flows and you want an explicit local commit boundary before push.

## Reset local to remote (destructive)

```bash
declarest repository reset
```

Use this only when you intentionally want to discard local uncommitted/unpushed changes.

## Discard local uncommitted changes only (destructive)

```bash
declarest repository clean
```

Use this to clean the local worktree (tracked edits and untracked files) without changing committed branch history.

## Recommended daily loop (git-backed repos)

```bash
declarest repository refresh
declarest repository status
declarest repository tree

declarest resource diff /corporations/acme
declarest resource apply /corporations/acme

declarest repository status
declarest repository history --oneline --max-count 5
declarest repository push
```

## CI/CD usage notes

- `filesystem` repos are good for ephemeral CI jobs that only need reconciliation behavior.
- `git` repos are better when the job also manages promotion branches or pushes generated updates.
- Run `repository status` before destructive or publish steps to surface divergence early.
- Use `repository history` to verify expected commit boundaries before pushing.
- Use `repository clean` when you need to remove local worktree noise before rerunning a workflow.
