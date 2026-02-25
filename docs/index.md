# DeclaREST

<p align="center">
  <img src="assets/logo.png" alt="DeclaREST logo" width="200">
</p>

DeclaREST lets you manage REST API resources as versioned files in a Git repository — bringing GitOps workflows to any system that offer tradicional HTTP API.

Instead of relying on scripts, *ad-hoc* `curl` commands, or manual UI clicks, you define the desired state in `json` or `yaml`, review changes via Git, and use the CLI to sync those files to the API (and back) in a repeatable, auditable way.

## Happy path (beginner flow)

1. Create a context (repository + resource-server auth).
2. Save one resource from the API into your repository.
3. Edit the local file.
4. Diff and apply.

```bash
declarest config add

declarest resource save /corporations/acme
# edit <repo>/corporations/acme/resource.json

declarest resource diff /corporations/acme
declarest resource apply /corporations/acme
```

## What makes it useful

- Git-friendly desired state for REST APIs
- Deterministic logical paths across environments
- Metadata to model non-REST or best-practices-drifting APIs
- Secret placeholders instead of plaintext in repository files
- Repeatable workflows for save/diff/apply/list/delete

## Start here

- [Installation](getting-started/installation.md)
- [Quickstart](getting-started/quickstart.md)
- [Concepts overview](concepts/overview.md)

## When your API is weird (and most are)

DeclaREST separates the **logical path** users operate on from the real HTTP endpoint shape.
Use metadata to:

- map a logical collection to a different API endpoint
- override per-operation endpoints and methods
- reshape payloads
- filter noisy list endpoints into deterministic resource lists

Start with [Metadata overview](concepts/metadata.md), then go deep on:

- [Metadata Overrides](concepts/metadata-overrides.md)
- [Custom Paths](concepts/metadata-custom-paths.md)
- [Advanced Metadata Configuration workflow](workflows/advanced-metadata-configuration.md)
