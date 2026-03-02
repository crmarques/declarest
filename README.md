# DeclaREST

<p align="center">
  <img src="docs/assets/logo.png" alt="DeclaREST logo" width="220">
</p>

<p align="center">
  Declarative resource sync between Git and REST APIs
</p>

DeclaREST turns REST API resources into versioned desired-state files you can review in Git and reconcile through a CLI.

## Project objective (10 seconds)

- Use stable logical paths (`/corporations/acme`) instead of raw endpoint shapes.
- Keep desired state in `resource.json|yaml` files inside a repository.
- Reconcile safely in both directions: `save` from API, `diff`, then `apply` back.

## How it works

![DeclaREST architecture](docs/assets/architecture.png)

1. A **context** defines repository, managed-server, auth, and optional metadata/secret providers.
2. **Metadata** maps logical paths to real API paths/methods/transforms.
3. The **CLI** runs deterministic workflows for read, diff, and mutation.

## Fast happy path

```bash
declarest config add

declarest resource save /corporations/acme
# edit repository file

declarest resource diff /corporations/acme
declarest resource apply /corporations/acme
```

## Current capabilities

- Resource workflows: `get|list|save|diff|explain|apply|create|update|delete|edit|copy`
- Raw HTTP workflows: `resource request <method>` for targeted debugging or ad-hoc operations
- Metadata workflows: `get|resolve|render|infer|set|unset` with OpenAPI and bundle-aware defaults
- Secret safety workflows: detect/fix metadata, store/mask/resolve placeholders, safe-save guards
- Repository workflows: `status|tree|history|commit|refresh|push|reset|clean|check`
- Context workflows: template/validate/add/update/resolve with runtime override support

## Install

Use a release binary or build locally:

```bash
go build -o bin/declarest ./cmd/declarest
./bin/declarest version
```

## Quickstart

```bash
declarest config add

declarest resource save /corporations/acme
declarest resource diff /corporations/acme
declarest resource apply /corporations/acme
```

If you prefer file-based context setup:

```bash
declarest config print-template > /tmp/contexts.yaml
# edit /tmp/contexts.yaml
declarest config add --payload /tmp/contexts.yaml --set-current
```

## Documentation map

Start here:

- `docs/index.md`
- `docs/getting-started/installation.md`
- `docs/getting-started/quickstart.md`
- `docs/concepts/overview.md`

Advanced modeling:

- `docs/concepts/metadata.md`
- `docs/concepts/metadata-overrides.md`
- `docs/concepts/metadata-custom-paths.md`
- `docs/workflows/advanced-metadata-configuration.md`
- `docs/workflows/custom-api-modeling.md`

Reference:

- `docs/reference/configuration.md`
- `docs/reference/cli.md`

## Contributing

See `docs/contributing.md` for development, testing, docs, and release workflows.
