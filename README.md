# DeclaREST

<p align="center">
    <img src="docs/assets/logo.png" alt="Logo" width="200">
</p>

DeclaREST is a Go CLI that keeps a Git-backed resource repository (desired state) in sync with remote REST APIs (actual state).

## Why use it

- Turn manual API calls into versioned, reviewable files in Git.
- Detect and reconcile drift between repository definitions and live systems.
- Switch safely between environments using named contexts.
- Keep secrets out of repository files while still templating them.

## Objectives

- Treat Git as the source of truth for apply/update/delete operations.
- Treat the remote API as the source of truth for refresh/get/list operations.
- Map repo paths to API endpoints deterministically using metadata.
- Make reconciliation repeatable and safe.

## Core concepts

- Resource repository: directories of logical paths, each resource stored as `resource.json`.
- Logical path: `/a/b/c` maps to a repo directory and a remote collection endpoint.
- Metadata: `metadata.json` files describe API paths, IDs, filters, and secrets.
- Context: named configuration for repository + managed server settings.
- Secret store: file-backed store for secrets referenced by repo files.

## Prerequisites

- Go toolchain (version in `go.mod`).
- Git is optional; DeclaREST uses go-git. Install Git only if you want to run Git commands yourself.
- Network access to the managed REST API.

## Quick start

1) Build the CLI:

```bash
make build
./bin/declarest --help
```

If you do not use `make`:

```bash
go build -o bin/declarest ./cli
```

2) Create a context (interactive):

```bash
./bin/declarest config add staging
./bin/declarest config list
```

Or generate a full config file:

```bash
./bin/declarest config print-template > ./contexts/staging.yaml
./bin/declarest config add staging ./contexts/staging.yaml
./bin/declarest config use staging
```

3) Initialize the repository and pull a resource:

```bash
./bin/declarest repo init
./bin/declarest resource get --path /teams/platform/users/alice --save
```

4) Edit repo files and reconcile back:

```bash
./bin/declarest resource diff --path /teams/platform/users/alice
./bin/declarest resource apply --path /teams/platform/users/alice
```

## Repository layout

```
/teams/
  _/metadata.json
  platform/
    users/
      _/metadata.json
      alice/
        resource.json
        permissions/
          _/metadata.json
          admin/
            resource.json
```

Other example logical paths: `/teams/platform/users/alice/permissions/admin`, `/teams/platform/permissions/admin`.

## Key commands

- `declarest config ...` manage contexts and configuration.
- `declarest repo ...` init/refresh/push/reset/check the resource repository.
- `declarest resource ...` get/list/diff/apply resources.
- `declarest metadata ...` view/update metadata for collections or resources.
- `declarest secret ...` store and audit secrets separately.

## Learn more

- Documentation (MkDocs): `docs/index.md`
- Getting started: `docs/getting-started/quickstart.md`
- Workflows (real example): `docs/workflows/sync.md`
- Detailed design and behavior: `specs/specs.md`
- Optional Keycloak e2e harness: `tests/managed-server/keycloak/README.md`
