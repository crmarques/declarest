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

The CLI source lives in `cli/`. Core logic is under `internal/`.

## Project structure

- `cli/`: Cobra commands and CLI entrypoint.
- `internal/`: core packages (reconciler, metadata, repository, secrets).
- `specs/`: detailed specification for behavior and file layout.
- `tests/`: integration and e2e harnesses.

## End-to-end tests

An optional Keycloak harness is available under `tests/keycloak`.
It provisions a temporary stack and runs a full lifecycle against the CLI.

```bash
./tests/keycloak/run-e2e.sh --repo-type fs
```

See `tests/keycloak/README.md` for prerequisites and options.

## Documentation

Docs are built with MkDocs and published to GitHub Pages.

```bash
pip install mkdocs
mkdocs serve
```

Site configuration lives in `mkdocs.yml` and content in `docs/`.

## Contribution workflow

1. Create a feature branch.
2. Make your changes and add tests when needed.
3. Run `make test` and ensure docs build if you touched docs.
4. Open a pull request with a clear summary and rationale.
