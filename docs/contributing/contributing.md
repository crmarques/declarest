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

Docs are built with MkDocs Material and the MkDocs Macros plugin, which exposes the current DeclaREST release version to examples in the docs.

```bash
pip install -r docs/requirements.txt
mkdocs serve
mkdocs build --strict
```
`make docs` prepares `.venv`, installs or refreshes `docs/requirements.txt`, and then runs `mkdocs build --strict --clean --site-dir .docs`.

Files:

- site config: `mkdocs.yml`
- content: `docs/`

GitHub Pages is published via `.github/workflows/docs.yml`.

## Release workflows

### Release automation

Tagged releases are coordinated by `.github/workflows/release.yml`. A `vX.Y.Z` tag drives one ordered workflow that validates the repository, publishes the operator image, regenerates and publishes the OLM bundle/catalog images for `X.Y.Z`, then runs GoReleaser for the CLI release and attached install assets.

Manual `workflow_dispatch` runs of the release workflow are snapshot smoke tests. They build the same asset set with manual prerelease versions but do not publish images or create a tagged release.

### Operator container image

Operator release images are published to GHCR by `.github/workflows/release.yml`.
For semver tags (`vX.Y.Z`), the release workflow publishes:

- `ghcr.io/crmarques/declarest-operator:vX.Y.Z`
- `ghcr.io/crmarques/declarest-operator:X.Y.Z`
- `ghcr.io/crmarques/declarest-operator:latest`

It also publishes:

- `ghcr.io/crmarques/declarest-operator-bundle:X.Y.Z`
- `ghcr.io/crmarques/declarest-operator-catalog:X.Y.Z`
- `ghcr.io/crmarques/declarest-operator-bundle:latest`
- `ghcr.io/crmarques/declarest-operator-catalog:latest`

The operator image, bundle image, catalog image, and CLI release assets are published with provenance attestations. Container images also request BuildKit provenance and SBOM attestations. External GitHub Actions used by release and validation workflows are pinned to full commit SHAs.

`.github/workflows/operator-image.yml` and `.github/workflows/bundle-image.yml` are manual smoke-build workflows only; tag pushes do not use them for publishing.

When docs are built from a release tag, examples use that tag's DeclaREST version automatically. For local builds you can override the rendered version with `DECLAREST_DOCS_VERSION=<version>`.

## E2E component system

The `test/e2e` directory contains a component-based E2E harness.

### Entry points

- Main runner: `test/e2e/run-e2e.sh`
- Fast Bash contract tests: `test/e2e/tests/run.sh`
- Component contract reference: `test/e2e/components/STANDARD.md`

### Component categories

Components live under `test/e2e/components/` and are grouped by concern:

- `managed-service/`
- `repo-type/`
- `git-provider/`
- `secret-provider/`

The harness composes these into a runnable profile/environment.

### Adding a component

Read these files first:

1. `test/e2e/components/STANDARD.md`
2. `test/e2e/lib/components_catalog.sh`
3. `test/e2e/lib/components_runtime.sh`
4. `test/e2e/lib/components_validate.sh`

Validation:

```bash
./test/e2e/run-e2e.sh --validate-components
./test/e2e/tests/run.sh
```

Keep component definitions deterministic and shellcheck-friendly. Prefer reusable hooks/scripts under the component directory. Update `STANDARD.md` when the component contract changes.

## Contribution checklist

1. Make the smallest coherent change.
2. Run the fastest checks that cover the changed behavior.
3. Run `mkdocs build --strict` for docs changes.
4. Review the diff for accidental secrets or generated noise.
5. Open a PR with rationale and verification notes.
