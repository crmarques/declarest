# Syncing resources

This page walks through common sync flows between the repository and the remote API.

## Pull remote state into the repository

```bash
./bin/declarest resource get --path /projects/example --save
```

For collection paths, DeclaREST saves each item as a separate resource by default.
To store the full collection response as one file:

```bash
./bin/declarest resource get --path /projects --save --save-as-one-resource
```

## Apply repository state to the API

```bash
./bin/declarest resource diff --path /projects/example
./bin/declarest resource apply --path /projects/example
```

Use `--sync` to refresh the local file after apply:

```bash
./bin/declarest resource apply --path /projects/example --sync
```

## Create or update only

- Create: `declarest resource create --path /projects/example`
- Update: `declarest resource update --path /projects/example`

## List resources

```bash
# From repository
./bin/declarest resource list --repo

# From remote
./bin/declarest resource list --remote
```

## Delete resources

```bash
# Remote only
./bin/declarest resource delete --path /projects/example --remote

# Repo only
./bin/declarest resource delete --path /projects/example --repo
```

Deletion prompts for confirmation by default.
