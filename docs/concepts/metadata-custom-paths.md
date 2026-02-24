# Custom Paths

This page covers the most important advanced capability in DeclaREST: modeling APIs whose endpoint layout does not match your desired logical path layout.

## The key idea

Your **logical path** is for humans and Git.
Your **API endpoint path** is whatever the server requires.
Metadata bridges the two.

The main tools are:

- `resourceInfo.collectionPath`
- `operationInfo.<operation>.path`
- `operationInfo.<operation>.httpMethod`
- payload transforms (`jqExpression`, suppress/filter attributes)

## `resourceInfo.collectionPath`

`collectionPath` defines the real API collection endpoint for a logical collection.
It can be templated.

Example:

```json
{
  "resourceInfo": {
    "collectionPath": "/admin/realms/{{.realm}}/components",
    "idFromAttribute": "id",
    "aliasFromAttribute": "name"
  }
}
```

This lets a logical path like `/admin/realms/prod/user-registry/ldap-main` map to Keycloak's `/components` endpoint.

## Relative operation paths (recommended)

Operation `path` values can be relative to the rendered `collectionPath`:

- `.` -> collection endpoint itself
- `./{{.id}}` -> child resource under the collection
- `./execution` -> nested sub-endpoint under the collection

This keeps metadata readable and avoids repeating long API paths.

## Operation path defaults

When you omit an operation `path`, DeclaREST uses safe defaults:

- `create` and `list`: `.`
- `get`, `update`, `delete`, `compare`: `./{{.id}}`

That means you often only override paths for the operations that truly differ.

## Keycloak example: logical `user-registry` backed by `/components`

Real fixture example (simplified from `test/e2e/.../user-registry/_/metadata.json`):

```json
{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "name",
    "collectionPath": "/admin/realms/{{.realm}}/components",
    "secretInAttributes": ["config.bindCredential[0]"]
  },
  "operationInfo": {
    "listCollection": {
      "payload": {
        "jqExpression": "[ .[] | select(.providerId == \"ldap\") ]"
      }
    }
  }
}
```

What this achieves:

- Users work with `/admin/realms/<realm>/user-registry/<name>`.
- DeclaREST actually calls `/admin/realms/<realm>/components`.
- List responses are filtered to LDAP components only.
- The repository stays organized by intent (`user-registry`) instead of raw backend endpoint (`components`).

## Keycloak example: operation override on executions

A second fixture (`.../authentication/flows/_/executions/_/metadata.json`) overrides operation paths and payload shape.

Key patterns:

- create uses `./execution` instead of the normal `.`
- update uses `./` (API-specific behavior)
- delete uses a different absolute endpoint and explicit `DELETE`
- payload `jqExpression` and `suppressAttributes` reshape the request body

This is exactly the kind of API drift metadata is designed to absorb.

## Path template context and indirection

Templates can resolve values from:

- current resource payload fields
- ancestor resource payload fields
- logical path context (`realm`, aliases, IDs, etc.)

This means `collectionPath` templates can still work even when the current payload does not include a field directly, as long as the logical path/ancestor context provides it.

## Validation loop for custom path modeling

Use this loop whenever you add or change path overrides:

```bash
# 1) inspect effective metadata
declarest metadata get /admin/realms/prod/user-registry/ldap-main

# 2) inspect just your overrides
declarest metadata get /admin/realms/prod/user-registry/ldap-main --overrides-only

# 3) render concrete operation specs
declarest metadata render /admin/realms/prod/user-registry/ldap-main get
declarest metadata render /admin/realms/prod/user-registry/ldap-main update

# 4) inspect the full plan
declarest resource explain /admin/realms/prod/user-registry/ldap-main
```

## Modeling guidelines for best-practices-drifting APIs

- Keep logical paths stable and human-friendly.
- Use `collectionPath` to point at the real backend endpoint.
- Prefer relative operation paths to minimize duplication.
- Use list `jq` filters to split mixed-type endpoints into logical collections.
- Use payload transforms to adapt request/response schema drift.
- Keep overrides minimal and layered; avoid copy-pasting full metadata blocks.
