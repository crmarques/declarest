# DeclaREST

<p align="center">
  <img src="docs/assets/logo.png" alt="DeclaREST logo" width="220">
</p>

<p align="center">
  Declarative resource sync between Git and REST APIs
</p>

DeclaREST lets you manage REST API resources as versioned files in a Git repository — bringing GitOps workflows to any system that offer tradicional HTTP API.

Instead of relying on scripts, *ad-hoc* `curl` commands, or manual UI clicks, you define the desired state in `json` or `yaml`, review changes via Git, and use the CLI to sync those files to the API (and back) in a repeatable, auditable way.

## Why this project exists

Many teams want GitOps-style workflows for systems that are only exposed through REST APIs.
That usually turns into:

- one-off scripts
- manual UI changes
- hard-to-review diffs
- drift between environments
- secrets accidentally copied into repos

DeclaREST solves that by giving you:

- a equivalent **logical path** model (`/corporations/acme`)
- a **repository layout** for desired state files
- **metadata** to map those paths to real API endpoints (even weird ones)
- **secret placeholders** so plaintext credentials do not need to live in Git

## 30-second mental model

You work with paths like this:

- `/corporations/acme`
- `/users/user-001`

Then DeclaREST can:

1. read the resource from the remote API and save it locally
2. let you edit the local file in Git
3. diff local vs remote
4. apply local desired state back to the API

```bash
declarest resource save /corporations/acme
# edit repository file

declarest resource diff /corporations/acme
declarest resource apply /corporations/acme
```

## What makes DeclaREST different

### Beginner-friendly happy flow

You can start with a single resource and a single context:

- create a context
- save one resource
- edit the file
- apply the change

### Handles real-world APIs (including messy ones)

DeclaREST is built for APIs that drift from REST best practices.
Metadata lets you:

- rename logical paths for better repository organization
- map logical collections to different backend endpoints
- override create/update/delete paths per operation
- reshape payloads with jq transforms
- filter mixed-type list endpoints into stable logical collections

This is especially useful for APIs where a clean logical hierarchy may not match the raw endpoint structure.

### REST API adapter

Because it acts as a REST API adapter, DeclaREST lets you import, transform, and operate on any endpoint without rewriting existing APIs.

## Quick start

### 1. Install

Download a release binary (recommended) or build from source.

```bash
go build -o bin/declarest ./cmd/declarest
./bin/declarest version
```

### 2. Create a context

```bash
declarest config create
```

Or generate and edit a template:

```bash
declarest config print-template > /tmp/contexts.yaml
declarest config add --payload /tmp/contexts.yaml --set-current
```

### 3. Save, diff, apply

```bash
declarest resource save /corporations/acme
declarest resource diff /corporations/acme
declarest resource apply /corporations/acme
```

## Documentation

### Start here (simple)

- `docs/index.md`
- `docs/getting-started/installation.md`
- `docs/getting-started/quickstart.md`
- `docs/concepts/overview.md`

### Go deeper (advanced metadata and custom APIs)

- `docs/concepts/metadata.md`
- `docs/concepts/metadata-overrides.md`
- `docs/concepts/metadata-custom-paths.md`
- `docs/workflows/advanced-metadata-configuration.md`
- `docs/workflows/custom-api-modeling.md`

### Reference

- `docs/reference/configuration.md`
- `docs/reference/cli.md`

## Example use cases

- manage system configuration (through its admin REST API) as versioned files
- standardize API-managed configuration across environments
- review and promote changes through Git workflows
- keep secrets in a secret store while preserving declarative resource definitions

## Contributing

See `docs/contributing.md` for development, test, docs, and release workflow notes.

## Status

The repository includes a growing CLI, metadata engine, and E2E harness with real API component fixtures to validate advanced behaviors.
If you have a difficult API shape, DeclaREST is designed to model it rather than forcing your repository to mirror the API's quirks.
