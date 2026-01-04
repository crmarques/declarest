# Repository operations

DeclaREST supports filesystem, git-local, and git-remote repositories.
These commands manage repository lifecycle and sync.

## Initialize

```bash
declarest repo init
```

This initializes the local repository and, if configured, the remote repository.

## Refresh from remote

```bash
declarest repo refresh
```

This fast-forwards the local repository to match the remote branch.

## Push to remote

```bash
declarest repo push
```

Use `--force` to rewrite remote history (confirmation required).

## Reset local to remote

```bash
declarest repo reset
```

This discards local changes and hard-resets to the remote branch.

## Check connectivity

```bash
declarest repo check
```

## Auto sync behavior

For git-remote repositories, `auto_sync` controls whether local changes are pushed automatically.
When unset, auto sync is enabled by default.
