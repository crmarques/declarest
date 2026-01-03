# Repository

The resource repository is the contract between DeclaREST and your API.
It is your **desired state**: JSON files in a deterministic directory layout that you can review and version.

## Repository types

- `filesystem`: a plain directory on disk.
- `git`: a local git repository, optionally configured with a remote URL.

DeclaREST uses `go-git` for repository operations, so the `git` CLI is optional (install it only if you want to run Git commands yourself).

## Where the repository lives

The repository base directory is defined in your active context config (see `declarest config print-template`).

## Repository layout

- Every **resource** lives in its own directory at `<logical-path>/`.
- The desired payload for that resource is stored as `<logical-path>/resource.json`.
- A **collection** is any directory path (for example `/notes/` or `/teams/platform/members/`).
- Collections can optionally be saved as a single `resource.json` file too (for example saving `/notes/` as `notes/resource.json`).

### Layout examples

Notes / tags:

```
/notes/
  _/metadata.json
  n-1001/
    resource.json
    tags/
      _/metadata.json
      meeting/
        resource.json
```

Teams → members → roles:

```
/teams/
  _/metadata.json
  platform/
    members/
      _/metadata.json
      xxx/
        roles/
          _/metadata.json
          admin/
            resource.json
```

## Key files and folders

- `resource.json` is the desired payload for a resource.
- `metadata.json` inside a resource directory overrides metadata for that resource only.
- `_/metadata.json` applies to an entire collection subtree.
- `_` is a reserved directory name used for metadata folders and wildcard matching.

See [Resource paths](resource-paths.md) for path rules, and [Metadata](metadata.md) for how DeclaREST merges and applies metadata.

## Repository commands (git-backed repos)

When using a git-backed repository, these commands help manage the local/remote sync:

- `declarest repo init`: initialize or clone the repository into the configured base directory.
- `declarest repo refresh`: fast-forward/pull remote changes into the local repo.
- `declarest repo push`: push local commits to the configured remote.
- `declarest repo reset`: reset local state to match remote (destructive).
