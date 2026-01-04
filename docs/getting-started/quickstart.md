# Quickstart

This walkthrough sets up a context, initializes a repository, and syncs a resource.

## 1) Set up a *Context*

a) Interactive setup:

```bash
declarest config init
```

b) Or generate a full config file:

```bash
declarest config print-template > /tmp/staging.yaml
```

Edit the `/tmp/staging.yaml` replacing the placeholders, then run:

```bash
declarest config add staging /tmp/staging.yaml
declarest config use staging
```

## 2) Check configuration

```bash
declarest config check
```

## 3) Init repository

```bash
declarest repo init
```

## 4) Pull a resource from your managed server into Git repository

```bash
declarest resource get --path /teams/platform/users/alice --save
```

This creates a `resource.json` under the repository base directory at:

```
<repo_base_dir>/teams/platform/users/alice/resource.json
```

## 5) Apply changes back to the API

Edit the local `resource.json`, then:

```bash
declarest resource diff --path /teams/platform/users/alice
declarest resource apply --path /teams/platform/users/alice
```

## Next

For more details, see [Concepts](../concepts/overview.md) and [Configuration](../reference/configuration.md).

For a complete, real example (Keycloak + GitLab + Vault), see [Syncing resources](../workflows/sync.md).

