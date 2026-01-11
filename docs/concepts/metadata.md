# Metadata

Metadata is what makes a repository path behave like an API resource.
It drives how logical paths map to API endpoints, how resources are compared, and how payloads are shaped.
Metadata is stored as JSON in `metadata.json` files.

## What metadata controls

- **URL mapping:** which collection endpoint to use and how to build per-resource URLs.
- **IDs and directory names:** how DeclaREST derives `id` and `alias` for a resource.
- **Listing:** how to query and filter a collection response.
- **Diff and payload shaping:** which attributes to ignore, filter, or suppress.
- **Secrets:** which attributes should be treated as secrets.

## Where metadata lives

DeclaREST supports two scopes:

- **Collection subtree metadata:** `<collection>/_/metadata.json` (applies to everything under that collection)
- **Resource-only metadata:** `<logical-path>/metadata.json` (applies only to that specific resource directory)

The `declarest metadata ...` commands default to treating paths as collections (so they update `_/metadata.json`).
To edit `metadata.json` for a specific resource directory, use `--for-resource-only`.

## Metadata layering

Metadata is merged in this order (later wins):

1. Defaults
2. Generic metadata from each ancestor collection (`_/metadata.json`)
3. Resource metadata (`<logical-path>/metadata.json`)

Merge behavior:

- Objects merge recursively.
- Scalars overwrite earlier values.
- Arrays replace earlier arrays (no deep merge).

### Wildcards

Metadata paths can include `_` as a wildcard segment (resource paths cannot).
This lets you define one set of rules that applies to many IDs.

For example, to apply the same metadata to every permission assignment under `/teams/platform/users/<id>/permissions/`,
store a collection metadata file at:

- `teams/platform/users/_/permissions/_/metadata.json`

When multiple metadata files match, the most specific ones (deeper paths, fewer wildcards) win.

## Minimal example (users)

For a user at logical path `/teams/platform/users/alice`, a typical REST layout is:

- Collection endpoint: `/api/v1/teams/platform/users`
- Resource endpoint: `GET /api/v1/teams/platform/users/alice`

Collection metadata could start as:

```json
{
  "resourceInfo": {
    "collectionPath": "/api/v1/teams/platform/users",
    "idFromAttribute": "username",
    "aliasFromAttribute": "username"
  }
}
```

Save this as `teams/platform/users/_/metadata.json` to apply it to the whole `/teams/platform/users/` subtree.

DeclaREST provides defaults for common CRUD operations, and you can override them under `operationInfo` as needed.

## Schema overview

```json
{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "id",
    "collectionPath": "/api/v1/teams/platform/users",
    "secretInAttributes": ["credentials.password", "sshKeys[0]"]
  },
  "operationInfo": {
    "getResource": {
      "url": { "path": "./{{.id}}" },
      "httpMethod": "GET",
      "httpHeaders": [
        "Accept: application/json"
      ]
    },
    "createResource": {
      "url": { "path": "." },
      "httpMethod": "POST",
      "httpHeaders": []
    },
    "updateResource": {
      "url": { "path": "./{{.id}}" },
      "httpMethod": "PUT",
      "httpHeaders": [],
      "payload": {
        "suppressAttributes": ["status"]
      }
    },
    "deleteResource": {
      "url": { "path": "./{{.id}}" },
      "httpMethod": "DELETE",
      "httpHeaders": []
    },
    "listCollection": {
      "url": {
        "path": ".",
        "queryStrings": ["expand=true"]
      },
      "httpMethod": "GET",
      "httpHeaders": [],
      "jqFilter": "."
    },
    "compareResources": {
      "ignoreAttributes": [],
      "suppressAttributes": [],
      "filterAttributes": [],
      "jqExpression": ""
    }
  }
}
```

## Template placeholders

Template placeholders use Go template syntax (for example `{{.id}}`).

- `{{.id}}` is derived from `resourceInfo.idFromAttribute` when present, otherwise it falls back to the last logical path segment.
- `{{.alias}}` follows the same rule using `resourceInfo.aliasFromAttribute`.
- The template context also includes the merged JSON attributes from the current resource and its ancestors, so you can reference fields like `{{.team}}` if they exist in your resource data.

For nested APIs, you can also reference ancestor resource attributes via relative placeholders like `{{../../id}}` (go up two segments, then read the `id` attribute from that ancestor resource).

## resourceInfo fields

- `idFromAttribute`: when present, uses this attribute as the remote id.
- `aliasFromAttribute`: when present, uses this attribute as the repo directory name.
- `collectionPath`: absolute collection endpoint (can include template placeholders).
- `secretInAttributes`: list of JSON attribute paths treated as secrets.

## operationInfo fields

Each operation can define:

- `url.path`: relative to `collectionPath` (`.` for the collection itself).
- `url.queryStrings`: list of query string fragments (for example `"expand=true"`).
- `httpMethod`: HTTP verb.
- `httpHeaders`: header list as `"Header: value"` strings or `{name, value}` entries.
- `payload`: optional payload shaping rules (suppress or filter attributes, or apply jq).
- `jqFilter`: optional jq filter for collection listings.
- `compareResources.*`: rules for ignoring or filtering attributes during diff.

## Tips

- Use `declarest metadata get` to inspect effective metadata.
- Use `declarest metadata set` and `declarest metadata add` to update metadata files.
- Run `declarest metadata update-resources` after changing alias/id rules.

## Metadata inference

`declarest metadata infer` walks the configured OpenAPI spec and suggests `resourceInfo` attributes. In addition to examining the collection/resource request schema and property names, inference now also looks at the child resource path parameters when the collection schema does not expose the identifier fields. For example, `/admin/realms/` can now infer both `idFromAttribute` and `aliasFromAttribute` as `realm` by detecting the `{realm}` parameter on `/admin/realms/{realm}` before you add metadata files or flags.
