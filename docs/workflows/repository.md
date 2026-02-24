# Repository Operations

This page covers repository lifecycle commands and how to use them safely in local and CI workflows.

## Check repository state first

```bash
declarest repo status
declarest repo check
```

Use this before any automated apply/push pipeline.

## Initialize the repository

```bash
declarest repo init
```

Behavior depends on context configuration:

- `filesystem`: prepares/uses a local directory
- `git`: initializes or syncs the local Git repo according to config

## Refresh local from remote (git repos)

```bash
declarest repo refresh
```

Use this before editing/applying if other automation or users might have pushed changes.

## Push local changes (git repos)

```bash
declarest repo push
```

If you intentionally need a non-fast-forward push, use the explicit force-push option exposed by the CLI and verify the remote state first.

## Reset local to remote (destructive)

```bash
declarest repo reset
```

Use this only when you intentionally want to discard local uncommitted/unpushed changes.

## Recommended daily loop (git-backed repos)

```bash
declarest repo refresh
declarest repo status

declarest resource diff /corporations/acme
declarest resource apply /corporations/acme

declarest repo status
declarest repo push
```

## CI/CD usage notes

- `filesystem` repos are good for ephemeral CI jobs that only need reconciliation behavior.
- `git` repos are better when the job also manages promotion branches or pushes generated updates.
- Run `repo status` before destructive or publish steps to surface divergence early.
