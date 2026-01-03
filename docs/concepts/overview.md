# Concepts overview

Think of DeclaREST as a deterministic “path mapper” plus a reconciler:

1. You refer to "things" by **paths** like `/teams/teamA/users/userA`.
2. That path maps both to a *REST API server* location as to a directory in your *resource repository*.
3. You can perform operations over these resources with DeclaREST CLI.

A common workflow looks like:

```bash
# download the resource into Git repository
declarest resource get --path /teams/teamA/users/userA --save

# you may want to edit that resource with your preferred tool
vi <base_repo_dir>/teams/teamA/users/userA/resource.json

# check resource states differences between remote server and git repository
declarest resource diff --path /teams/teamA/users/userA

# update remote resource server with new desired state
declarest resource apply --path /teams/teamA/users/userA
```

## Core terms

- **Resource:** one remote entity represented locally as JSON (`<logical-path>/resource.json`).
- **Path:** info used to localize a resource, both in remote server and in repository (for example `/teams/platform/members/xxx/roles/admin`).
- **Logical path:** a normalized, absolute path you use in CLI commands.
- **Collection:** a group of similar resources.
- **Metadata:** Set of information that describes a resource or collection.
- **Context:** named configuration for repository + managed server + secrets.
- **Secrets manager:** a separate store for sensitive values referenced by resource files.

## Next

See more details in [Repository](repository.md) for how paths map to files, and [Metadata](metadata.md) for how paths map to API endpoints.
