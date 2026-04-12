# Core Concepts

> Completed a [Quickstart](../getting-started/quickstart-cli.md)? This page explains the building blocks you just used.

DeclaREST manages REST API resources as declarative files.
You define desired state in a repository, and DeclaREST reconciles it against real API state through metadata-driven mappings.

## Two modes

### CLI (on-demand)

Run commands when you want explicit control:

```bash
declarest resource save  /corporations/acme   # import remote state
declarest resource diff  /corporations/acme   # compare desired vs real
declarest resource apply /corporations/acme   # reconcile
```

Good for local workflows, CI pipelines, and controlled rollouts.

### Operator (continuous)

A Kubernetes Operator that watches a Git repository and reconciles continuously:

1. Admin updates desired state in Git (often via CLI + pull request).
2. Operator detects the change and builds a sync plan.
3. Real API state converges to desired state.
4. Status conditions report success or drift.

This is the recommended runtime model. Most teams use both: **CLI for authoring, Operator for runtime**.

## Context

A context is a named configuration that ties everything together for one run:

- **Repository** -- where desired-state files live (filesystem or Git)
- **Managed Service** -- which API to target (URL, auth)
- **Secret Store** -- where sensitive values are kept (optional)
- **Metadata source** -- where metadata rules come from (optional)

Contexts live in `~/.declarest/configs/contexts.yaml`. The catalog also supports reusable credentials defined once and referenced across components.

```yaml
currentContext: dev

credentials:
  - name: shared
    username: api-user
    password:
      prompt: true
      persistInSession: true

contexts:
  - name: dev
    repository:
      git:
        local:
          baseDir: /work/repo
        remote:
          url: https://example.com/org/repo.git
          auth:
            basic:
              credentialsRef:
                name: shared
    managedService:
      http:
        url: https://api.example.com
        auth:
          basic:
            credentialsRef:
              name: shared
```

Key rules:

- `managedService.http.auth` must be exactly one of `oauth2`, `basic`, or `customHeaders`.
- `metadata` may define at most one of `baseDir`, `bundle`, or `bundleFile`.
- Runtime overrides (`--set key=value`) do not mutate the catalog file.

See [Configuration reference](../reference/configuration.md) for the full schema.

## Resources and collections

A **resource** is a logical object stored as `resource.<ext>` (JSON, YAML, XML, or binary), identified by a logical path:

```
/corporations/acme  ->  corporations/acme/resource.json
```

A **collection** groups resources under a trailing slash:

```
/customers/  ->  customers/_/metadata.json   (collection metadata)
                 customers/acme/resource.json (child resource)
                 customers/beta/resource.json (child resource)
```

Resource files can also use:

- **Includes**: {% raw %}`{{include config.json}}`{% endraw %} embeds a sibling file's content into the payload.
- **Defaults**: shared field values managed through metadata, so `resource.<ext>` only stores overrides.

## Logical paths, selectors, and wildcards

### Logical paths

All CLI commands use **logical absolute paths**:

| Path | Type | Meaning |
|------|------|---------|
| `/corporations/acme` | resource | one object |
| `/customers/` | collection | a group of resources |

Rules: must start with `/`, segments separated by `/`, trailing `/` = collection.

### Selectors

Metadata can target patterns using `_` as a wildcard segment:

- `/customers/_/addresses/_/` means "any address collection under any customer"

Selectors define reusable metadata rules. They are different from command wildcards.

### Command wildcards

In `resource save`, `_` expands remote collections:

```bash
declarest resource save /customers/_/addresses/_
```

This discovers and saves all concrete resources matching the pattern.

### Metadata file locations

- **Collection subtree**: `customers/_/metadata.json` -- applies to all resources under `/customers/`
- **Resource-only**: `customers/acme/metadata.json` -- applies only to `/customers/acme`

## Repository

The repository is the desired-state store. Two backends:

| Backend | Use case |
|---------|----------|
| `filesystem` | Local directory only. Simplest option for testing and CI. |
| `git` | Local Git repo with optional remote push/refresh/reset. Required for Operator mode. |

Both use the same file layout:

```text
corporations/acme/resource.json       # payload
corporations/acme/metadata.json       # optional resource override
corporations/_/metadata.json          # optional collection defaults
```

Metadata can live in a separate directory via `metadata.baseDir`, useful when payloads and rules come from different sources.

Core commands: `repository status`, `init`, `refresh`, `push`, `reset`, `clean`.

See [Repository and Git Workflows](repository-and-git-workflows.md) for day-to-day operations.

## Managed Service

Defines how DeclaREST connects to the target API:

- `http.url` -- base URL
- `http.auth` -- one of `oauth2`, `basic`, or `customHeaders`
- Optional: `tls`, `proxy`, `requestThrottling`, `openapi`

Auth and TLS are connectivity settings, not resource content. In Operator mode, credentials come from Kubernetes Secrets.

Keep one `ManagedService` per endpoint/auth profile. Multiple `SyncPolicy` resources can reference the same server with different source paths.

## Metadata at a glance

Metadata is the translation layer between logical paths and real API behavior. It controls:

| Area | What it does |
|------|--------------|
| **Identity** | `resource.id`, `resource.alias` -- how a resource is uniquely identified |
| **Path mapping** | `resource.remoteCollectionPath` -- maps logical paths to actual API endpoints |
| **Operations** | `operations.get`, `.create`, `.update`, `.delete`, `.list` -- HTTP method, path, query, headers, body per operation |
| **Transforms** | `operations.*.transforms` -- ordered mutation pipeline (jq, select/exclude attributes) |
| **Compare** | `compare.transforms` -- suppress noisy fields before diffing |
| **Secrets** | `resource.secretAttributes` -- which fields are sensitive |
| **Externalized** | `resource.externalizedAttributes` -- long text fields stored as sidecar files |

Metadata files are **overrides**, not full schemas. You store only what you need; DeclaREST merges them with defaults at runtime.

If your API is clean REST, you may need very little metadata. If your API is inconsistent or RPC-ish, metadata becomes the core modeling tool.

See [Metadata and API Modeling](metadata-and-api-modeling.md) for the full treatment.

## Secrets at a glance

DeclaREST keeps sensitive values out of Git:

1. **Metadata declares** which attributes are secrets (`resource.secretAttributes`).
2. **Payloads store placeholders**: {% raw %}`{{secret .}}`{% endraw %} or {% raw %}`{{secret custom-key}}`{% endraw %}.
3. **Secret store holds values** (encrypted file or HashiCorp Vault).
4. **Workflows resolve** placeholders before sending API requests.

```bash
# Import with automatic secret detection and masking
declarest resource save /corporations/acme --secret-attributes
```

See [Managing Secrets](managing-secrets.md) for the complete workflow.

## Git as source of truth

In DeclaREST GitOps:

- **Desired state** = what is declared in repository files.
- **Real state** = what the API currently returns.
- **Reconciliation** moves real state toward desired state.

### Pull request flow

1. Import or edit desired state.
2. Run `resource diff` to validate changes.
3. Open a PR for review.
4. Merge to the tracked branch.
5. Operator reconciles merged state.

### Environment patterns

**Branch-based**: separate branches (`main`, `staging`) with a `ResourceRepository` per branch. Use when branch protections drive promotion.

**Directory-based**: one branch, separate directories (`envs/dev/`, `envs/prod/`). Each `SyncPolicy.source.path` points at one directory. Use when path-level ownership is clearer.

### Guardrails

- Keep logical paths stable; avoid rename churn.
- Treat direct API edits as drift.
- Prefer PR review for production changes.
- Use small commits scoped to one logical area.

## Bundles

A bundle is a reusable package of metadata for a specific API product. Instead of writing metadata from scratch, you reference a pre-built bundle:

```yaml
metadata:
  bundle: keycloak-bundle-1.0.0.tar.gz
```

Available bundles include Keycloak and Rundeck. Bundles contain ready-made metadata trees, and optionally an OpenAPI spec, that map logical paths to the product's actual API.
