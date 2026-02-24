# Concepts Overview

DeclaREST is a mapping layer plus a reconciler that acts as a REST API adapter, exposing friendly logical paths while translating requests into whatever endpoints the target API demands.

- **Logical paths** are the user-facing addresses (`/corporations/acme`, `/admin/realms/prod/user-registry/ldap-main`).
- **Repository files** store desired state at deterministic paths.
- **Metadata** translates logical paths into the real API endpoints, methods, and payload transforms.
- **CLI workflows** read, diff, and apply resources using those rules.

## Mental model

Think in this order:

1. Choose a logical path that makes sense for humans and Git history.
2. Store payloads in repository files under that path.
3. Use metadata to adapt that path to whatever the API actually expects.
4. Use `resource save`, `resource diff`, and `resource apply` as the normal workflow.

## Core building blocks

### Context

A named configuration that combines:

- repository backend (`filesystem` or `git`)
- resource-server config (`base-url`, auth, optional OpenAPI)
- optional secret store
- optional metadata base-dir override

### Resource

A single logical object stored locally as:

- `resource.json`, or
- `resource.yaml` when `repository.resource-format: yaml`

### Collection

A logical grouping of resources, identified by a trailing slash:

- resource: `/corporations/acme`
- collection: `/customers/`

### Metadata

JSON directives that control:

- identity mapping (`idFromAttribute`, `aliasFromAttribute`)
- path mapping (`collectionPath`, operation `path`)
- HTTP behavior (`httpMethod`, headers, query)
- payload transforms (`jqExpression`, filter/suppress attributes)
- compare/diff normalization
- secret-marked attributes (`secretInAttributes`)

## Typical lifecycle

```bash
# Pull remote state into the repository
declarest resource save /corporations/acme

# Review local desired state
declarest resource diff /corporations/acme

# Push desired state back to the API
declarest resource apply /corporations/acme
```

## Where advanced APIs fit

Many APIs are not clean REST:

- IDs differ from user-facing names
- collections return mixed resource types
- create/update/delete endpoints use different shapes
- nested objects are stored in a flat backend endpoint
- list and get endpoints require different transforms

DeclaREST expects that and gives you metadata primitives to normalize the experience.

Start with:

- [Paths and Selectors](paths-and-selectors.md)
- [Metadata Overview](metadata.md)
- [Metadata Overrides](metadata-overrides.md)
- [Custom Paths](metadata-custom-paths.md)
