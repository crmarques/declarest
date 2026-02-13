# Contributing

Thanks for helping improve DeclaREST. This page covers local development, testing, and docs.

## Development setup

```bash
# Build the CLI
make build

# Run tests
make test

# Format code
make fmt
```

The CLI source lives in `cli/`. Core logic lives in top-level packages.

## Project structure

- `cli/`: Cobra commands and CLI entrypoint.
- `context/`, `managedserver/`, `metadata/`, `openapi/`, `reconciler/`, `repository/`, `resource/`, `secrets/`, `yamlutil/`: core packages.
- `specs/`: detailed specification for behavior and file layout.
- `tests/`: integration and e2e harnesses.

## End-to-end tests

E2E runs are orchestrated by the generic runner, which dispatches to a managed server harness.

```bash
# Automated end-to-end flow
./tests/run-tests.sh --e2e --managed-server keycloak --repo-provider git --secret-provider file

# check options:
./tests/run-tests.sh --help
```

By default `./tests/run-tests.sh` runs the `--complete` profile, which drives the full set of context/metadata/OpenAPI/main flow/variation groups. Use `--reduced` for the primary flow only (representative subset with a trimmed lifecycle; metadata/OpenAPI/variation groups are skipped) and `--skip-testing-*` flags to omit individual test groups (the runner prints aligned RUNNING/DONE/SKIPPED/FAILED statuses for each group).

See `tests/managed-server/keycloak/README.md` for Keycloak prerequisites and options.

### Test harness standard

To add a new managed server, follow the bash contract used by `tests/run-tests.sh`:

- Provide `tests/managed-server/<name>/run-e2e.sh` and `tests/managed-server/<name>/run-interactive.sh`.
- Accept `--managed-server`, `--repo-provider`, and `--secret-provider` (ignore unsupported values with a clear error or warning).
- Keep managed-server-specific assets under `tests/managed-server/<name>/scripts` and `tests/managed-server/<name>/templates`.
- Use shared folders for future reuse: `tests/repo-provider/<type>` and `tests/secret-provider/<type>`.

## Documentation

Docs are built with MkDocs and published to GitHub Pages.

```bash
pip install mkdocs mkdocs-material
mkdocs serve
```

Site configuration lives in `mkdocs.yml` and content in `docs/`.

## Contribution workflow

1. Create a feature branch.
2. Make your changes and add tests when needed.
3. Run `make test` and ensure docs build if you touched docs.
4. Open a pull request with a clear summary and rationale.
