# Custom API Modeling Recipes

This page collects reusable metadata patterns for APIs that drift from clean REST conventions.

## Recipe 1: Friendly aliases, opaque IDs

Problem:

- API endpoints use opaque IDs
- users want stable human-readable paths in Git

Use:

```json
{
  "resourceInfo": {
    "idFromAttribute": "id",
    "aliasFromAttribute": "name"
  }
}
```

Result:

- repo path segment uses `name`
- remote operations still render paths with `id`

## Recipe 2: Backend endpoint name differs from domain concept

Problem:

- backend endpoint is generic (`/components`)
- you want logical paths by intent (`/user-registry/`, `/mappers/`)

Use `resourceInfo.collectionPath` to point logical collections to the shared backend endpoint, then apply list `jq` filters to split types.

## Recipe 3: Create endpoint differs from update endpoint

Problem:

- create uses `/.../execution`
- update uses `/.../` or another path

Use per-operation overrides:

```json
{
  "operationInfo": {
    "createResource": { "path": "./execution" },
    "updateResource": { "path": "./" }
  }
}
```

## Recipe 4: Request payload field names differ by operation

Problem:

- API expects `provider` on create
- resource payload stores `providerId`

Use payload transforms:

```json
{
  "operationInfo": {
    "createResource": {
      "payload": {
        "jqExpression": ". | .provider = .providerId",
        "suppressAttributes": ["providerId"]
      }
    }
  }
}
```

## Recipe 5: List endpoint returns mixed object types

Problem:

- one endpoint returns many object categories
- your logical collection should contain only one category

Use `listCollection.payload.jqExpression` to filter deterministically.

```json
{
  "operationInfo": {
    "listCollection": {
      "payload": {
        "jqExpression": "[ .[] | select(.type == \"desired\") ]"
      }
    }
  }
}
```

## Recipe 6: Diff noise from server-generated fields

Problem:

- diffs are noisy due to timestamps, versions, computed status fields

Use compare transforms to suppress or filter fields before diffing.

Common targets:

- `updatedAt`
- `version`
- server-generated IDs inside nested status blocks

## Recipe 7: Nested logical resources backed by a flat backend collection

Problem:

- backend stores child objects flat with a parent ID field
- you want nested logical paths in the repo

Pattern:

- map logical child collection to the backend collection endpoint
- filter list results by parent ID
- optionally use `resource("<logical-path>")` inside list `jq` to resolve parent data

## Verification loop for every recipe

```bash
declarest metadata get <path>
declarest metadata render <path> get
declarest metadata render <path> update
declarest resource explain <path>
```

Do this before you run `resource apply`.
