# Contributing

This page covers local development, validation, and documentation workflows for contributors.

## Development setup

Common commands from the repository root:

```bash
make build
make test
make check
```

Other useful targets:

```bash
make fmt
make vet
make lint
make e2e-contract
make e2e-validate-components
```

## Project layout (high level)

- `cmd/declarest/` - CLI entrypoint
- `internal/` - internal CLI and provider implementations
- `agents/` - repository rebuild specs, references, and skills
- `docs/` - MkDocs content
- `test/e2e/` - E2E harness, components, and fixtures
- `.github/workflows/` - CI, release, and docs publishing automation

## Tests and validation

### Fast local checks

```bash
make check
```

### E2E harness

```bash
# Validate component contracts/fixtures only (fast)
make e2e-validate-components

# Run E2E workload (see flags in run-e2e.sh)
./test/e2e/run-e2e.sh --help
```

## Documentation workflow

Docs are built with MkDocs Material.

```bash
pip install mkdocs-material
mkdocs serve
mkdocs build --strict
```

Files:

- site config: `mkdocs.yml`
- content: `docs/`

GitHub Pages is published via `.github/workflows/docs.yml`.

## Release workflows

### CLI binaries

CLI release artifacts are built with GoReleaser via `.github/workflows/release.yml` and `.goreleaser.yaml`.

### Operator container image

Operator release images are published to GHCR via `.github/workflows/operator-image.yml`.
For semver tags (`vX.Y.Z`), the workflow publishes:

- `ghcr.io/crmarques/declarest-operator:vX.Y.Z`
- `ghcr.io/crmarques/declarest-operator:latest`

Tagging a semver version (for example `v1.2.3`) triggers both release flows.

## Contribution checklist

1. Make the smallest coherent change.
2. Run the fastest checks that cover the changed behavior.
3. Run `mkdocs build --strict` for docs changes.
4. Review the diff for accidental secrets or generated noise.
5. Open a PR with rationale and verification notes.
