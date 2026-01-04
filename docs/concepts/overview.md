# Concepts overview

Think of DeclaREST as a deterministic “path mapper” plus a reconciler:

1. You refer to "things" by **paths** like `/teams/platform/users/alice` or `/teams/platform/users/alice/permissions/admin`.
2. That path maps both to a *REST API server* location (`https://<hostname>/teams/platform/users/alice`) as to a directory in your *resource repository* (`<repo_base_dir>/teams/platform/users/alice`).
3. You can perform operations over these resources with DeclaREST CLI.

A common workflow looks like:

```bash
# download the resource into Git repository
declarest resource get --path /teams/platform/users/alice --save

# you may want to edit that resource with your preferred tool
vi <base_repo_dir>/teams/platform/users/alice/resource.json

# check resource states differences between remote server and git repository
declarest resource diff --path /teams/platform/users/alice

# update remote resource server with new desired state
declarest resource apply --path /teams/platform/users/alice
```

## Core terms

- **Resource:** one remote entity represented locally as JSON (`<logical-path>/resource.json`).
- **Path:** info used to localize a resource, both in remote server and in repository.
- **Logical path:** a normalized, absolute path you use in CLI commands (used specially when API REST drifts from REST standards. Check [Metadata](metadata.md) page for more details).
- **Collection:** a group of similar resources.
- **Metadata:** Set of information that describes resources or collections.
- **Context:** named configuration for repository + managed server + secrets store.
- **Secret store:** a separate store for sensitive values referenced by resource files.
