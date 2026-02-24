# Resource Files

A resource is a logical object stored in the repository as a JSON or YAML document.

## File naming

The active context controls the repository payload format:

- `repository.resource-format: json` -> `resource.json`
- `repository.resource-format: yaml` -> `resource.yaml`

## Repository layout

Examples for logical path `/corporations/acme`:

- payload: `customers/acme/resource.json`
- resource-only metadata (optional): `customers/acme/metadata.json`

Collection metadata for `/customers/`:

- `customers/_/metadata.json`

## Resource payloads vs collection payloads

DeclaREST can persist collection responses in two ways with `resource save`:

- `--as-items` (default for list payloads): each item becomes its own resource directory
- `--as-one-resource`: stores the collection payload as one resource file

Use `--as-items` for normal GitOps/reconciliation. Use `--as-one-resource` for opaque list endpoints or snapshots.

## Includes inside resource payloads

Resource files support the literal include directive:

- `{{include file.ext}}`

The included file path is resolved relative to the current resource directory.
Structured JSON/YAML content is merged as data; non-JSON/YAML content is included as text.

Example:

```yaml
service:
  config: "{{include config.json}}"
  script: "{{include deploy.sh}}"
```

This is useful when a resource embeds long scripts, policy documents, or nested config fragments.

## Secrets in resource files

Sensitive attributes should be stored as placeholders, not plaintext:

- `{{secret .}}`
- `{{secret custom-key}}`

The secret values live in the configured secret store, and the placeholder stays in Git.
See [Secrets](secrets.md).

## Inspecting resource behavior before apply

Use these commands together:

```bash
declarest resource get --source repository /corporations/acme
declarest metadata get /corporations/acme
declarest metadata render /corporations/acme update
```

That combination lets you inspect payload + metadata + rendered remote request mapping before a write.
