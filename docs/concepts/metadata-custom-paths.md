# Custom Paths

This page covers the most important advanced capability in DeclaREST: modeling APIs whose endpoint layout does not match your desired logical path layout.

## The key idea

Your **logical path** is for humans and Git.
Your **API endpoint path** is whatever the server requires.
Metadata bridges the two.

The main tools are:

- `resource.remoteCollectionPath`
- `operations.<operation>.path`
- `operations.<operation>.method`
- ordered `transforms` steps (`jqExpression`, `selectAttributes`, `excludeAttributes`)

## `resource.remoteCollectionPath`

`remoteCollectionPath` defines the managed-server collection endpoint for a logical collection.
When omitted, DeclaREST defaults it to the logical collection path.
It can be templated.

Use canonical JSON Pointer placeholders such as {% raw %}`{{/realm}}`{% endraw %} in docs and metadata inference output. Single-level shorthand such as {% raw %}`{{realm}}`{% endraw %} is also accepted when the lookup is just one token.

Example:

```json
{
  "resource": {
    "remoteCollectionPath": "{% raw %}/admin/realms/{{/realm}}/components{% endraw %}",
    "id": "{% raw %}`{{/id}}`{% endraw %}",
    "alias": "{% raw %}`{{/name}}`{% endraw %}"
  }
}
```

This lets a logical path like `/admin/realms/prod/user-registry/ldap-main` map to Keycloak's `/components` endpoint.

## Relative operation paths (recommended)

Operation `path` values can be relative to the rendered effective collection path:

- `.` -> collection endpoint itself
- {% raw %}`./{{/id}}`{% endraw %} -> child resource under the collection
- `./execution` -> nested sub-endpoint under the collection

This keeps metadata readable and avoids repeating long API paths.

## Operation path defaults

When you omit an operation `path`, DeclaREST uses safe defaults:

- `create` and `list`: `.`
- `get`, `update`, `delete`, `compare`: {% raw %}`./{{/id}}`{% endraw %}

That means you often only override paths for the operations that truly differ.

## Keycloak example: logical `user-registry` backed by `/components`

Real fixture example (simplified from `test/e2e/.../user-registry/_/metadata.yaml`):

```json
{
  "resource": {
    "id": "{% raw %}`{{/id}}`{% endraw %}",
    "alias": "{% raw %}`{{/name}}`{% endraw %}",
    "remoteCollectionPath": "{% raw %}/admin/realms/{{/realm}}/components{% endraw %}",
    "secretAttributes": ["/config/bindCredential/0"]
  },
  "operations": {
    "list": {
      "transforms": [
        { "jqExpression": "[ .[] | select(.providerId == \"ldap\") ]" }
      ]
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

A second fixture (`.../authentication/flows/_/executions/_/metadata.yaml`) overrides operation paths and payload shape.

Key patterns:

- create uses `./execution` instead of the normal `.`
- update uses `./` (API-specific behavior)
- delete uses a different absolute endpoint and explicit `DELETE`
- `transforms` reshapes the request body

This is exactly the kind of API drift metadata is designed to absorb.

## Path template context and indirection

Templates can resolve values from:

- current resource payload fields
- ancestor resource payload fields
- logical path context (`realm`, aliases, IDs, etc.)

This means `remoteCollectionPath` templates can still work even when the current payload does not include a field directly, as long as the logical path/ancestor context provides it.
Plural logical collection segments such as `/projects/<project>/...` also remain available as fallback template fields.

## Descendant-aware collection selectors

Some APIs expose one backend subtree while you want deeper logical folders beneath the same selector.
Use `selector.descendants: true` on collection metadata when that rule must keep applying below the matched collection root.

Rundeck key storage is a good example:

```json
{
  "selector": {
    "descendants": true
  },
  "resource": {
    "id": "{% raw %}`{{/name}}`{% endraw %}",
    "alias": "{% raw %}`{{/name}}`{% endraw %}",
    "remoteCollectionPath": "{% raw %}/storage/keys/project/{{/project}}{{/descendantCollectionPath}}{% endraw %}"
  },
  "operations": {
    "list": {
      "path": "."
    },
    "get": {
      "path": "{% raw %}./{{/id}}{% endraw %}"
    }
  }
}
```

With that selector:

- `/projects/platform/secrets/path/to/` renders list operations against `/storage/keys/project/platform/path/to`
- `/projects/platform/secrets/path/to/db-password` renders get/update/delete against `/storage/keys/project/platform/path/to/db-password`
- `{{/descendantCollectionPath}}` carries the nested collection suffix, while `{{/descendantPath}}` carries the full nested resource suffix

This keeps the logical repository tree readable without hardcoding every deeper folder level in metadata.

## Validation loop for custom path modeling

Use this loop whenever you add or change path overrides:

```bash
# 1) inspect effective metadata
declarest resource metadata get /admin/realms/prod/user-registry/ldap-main

# 2) inspect just your overrides
declarest resource metadata get /admin/realms/prod/user-registry/ldap-main --overrides-only

# 3) render concrete operation specs
declarest resource metadata render /admin/realms/prod/user-registry/ldap-main get
declarest resource metadata render /admin/realms/prod/user-registry/ldap-main update

# 4) inspect the full plan
declarest resource explain /admin/realms/prod/user-registry/ldap-main
```

## Modeling guidelines for best-practices-drifting APIs

- Keep logical paths stable and human-friendly.
- Use `remoteCollectionPath` to point at the real backend endpoint.
- Prefer relative operation paths to minimize duplication.
- Use list `jq` filters to split mixed-type endpoints into logical collections.
- Use `transforms` pipelines to adapt request/response schema drift.
- Keep overrides minimal and layered; avoid copy-pasting full metadata blocks.
