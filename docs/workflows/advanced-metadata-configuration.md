# Advanced Metadata Configuration

This page is the deep-dive example workflow for modeling non-trivial APIs.

It uses Keycloak metadata fixtures from `test/e2e/components/resource-server/keycloak/metadata/...` to demonstrate:

- custom logical paths backed by different API endpoints
- list filtering for mixed-type endpoints
- operation-specific endpoint overrides
- payload rewrites for create/update calls
- layered metadata (base rule + deeper overrides)

## Goal

Create a clean logical path model even when the API exposes a lower-level backend endpoint.

Keycloak example:

- desired logical path model: `/admin/realms/<realm>/user-registry/<provider>`
- real endpoint: `/admin/realms/<realm>/components`
- list endpoint returns multiple component types (LDAP, others)

## Example 1: Custom path + filtered list (`user-registry`)

Inspired by:

- `test/e2e/components/resource-server/keycloak/metadata/admin/realms/_/user-registry/_/metadata.json`

### Metadata

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

### What this does

- maps a logical `user-registry` collection to Keycloak's `/components`
- filters list responses down to LDAP providers only
- uses `name` as the friendly directory segment (alias)
- marks LDAP bind credential as a secret

### Verify the mapping

```bash
declarest metadata get /admin/realms/prod/user-registry/ldap-main
declarest metadata render /admin/realms/prod/user-registry/ldap-main get
declarest metadata render /admin/realms/prod/user-registry/ list
```

### Why this is powerful

It lets the repository represent the domain concept (`user-registry`) instead of the backend transport concept (`components`).
That is the core DeclaREST pattern for best-practices-drifting APIs.

## Example 2: Layered override for flow executions

Inspired by:

- `.../authentication/flows/_/metadata.json`
- `.../authentication/flows/_/executions/_/metadata.json`

The parent flow metadata sets identity defaults for flows.
A deeper metadata file for `executions` overrides specific operations and payloads.

### Base flow metadata (identity)

```json
{
  "resourceInfo": {
    "idFromAttribute": "alias",
    "aliasFromAttribute": "alias"
  }
}
```

### Executions metadata (operation overrides)

```json
{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "displayName"
  },
  "operationInfo": {
    "createResource": {
      "path": "./execution",
      "payload": {
        "jqExpression": ". | .provider = .providerId",
        "suppressAttributes": ["providerId"]
      }
    },
    "updateResource": {
      "path": "./"
    },
    "deleteResource": {
      "httpMethod": "DELETE",
      "path": "/admin/realms/{{.realm}}/authentication/executions/{{.id}}"
    }
  }
}
```

### What this does

- create uses a non-standard sub-endpoint (`./execution`)
- create payload copies `providerId` to `provider` and removes the original field
- update uses an API-specific collection-relative path (`./`)
- delete uses a completely different absolute endpoint shape

This is a textbook example of operation-level API drift handled cleanly by metadata.

### Verify each operation explicitly

```bash
declarest metadata render \"/admin/realms/prod/authentication/flows/browser/executions/OTP Form\" create
declarest metadata render \"/admin/realms/prod/authentication/flows/browser/executions/OTP Form\" update
declarest metadata render \"/admin/realms/prod/authentication/flows/browser/executions/OTP Form\" delete
```

If the logical path alias contains spaces, shell quoting matters.

## Example 3: Filtering by parent lookup with `resource()` in list `jq`

Inspired by:

- `.../user-registry/_/mappers/_/metadata.json`

Pattern (simplified): use a shared backend collection endpoint, but filter rows based on the parent resource's resolved ID.

Why this matters:

- the backend endpoint may return all mappers for all components
- your logical path should still model mappers under a specific parent provider

This is an advanced pattern for APIs that flatten nested resources internally.

## Authoring workflow (recommended)

1. Model the logical path tree you want users to operate on.
2. Add high-level selector metadata with `collectionPath` and identity fields.
3. Add list `jq` filters to isolate the logical collection.
4. Render operations with `metadata render` for concrete paths.
5. Add per-operation overrides only where the API deviates.
6. Use `resource explain` to validate reconciliation behavior end-to-end.
7. Save/apply one resource before scaling to a whole collection.

## Checklist for advanced metadata changes

- `metadata get` shows the expected merged result
- `metadata get --overrides-only` stays minimal and readable
- `metadata render` matches real API endpoints for get/create/update/delete/list
- `resource explain` resolves the correct alias/id and operation plan
- `resource save` / `resource apply` works on a single concrete path first
