# E2E Components (Contributor Reference)

This page summarizes the current `test/e2e` component system used by the repository E2E harness.

## Entry points

- Main runner: `test/e2e/run-e2e.sh`
- Fast Bash contract tests: `test/e2e/tests/run.sh`
- Component contract reference: `test/e2e/components/STANDARD.md`

## Component categories

Components live under `test/e2e/components/` and are grouped by concern:

- `resource-server/`
- `repo-type/`
- `git-provider/`
- `secret-provider/`

The harness composes these into a runnable profile/environment.

## What to read first when adding a component

1. `test/e2e/components/STANDARD.md`
2. `test/e2e/lib/components_catalog.sh`
3. `test/e2e/lib/components_runtime.sh`
4. `test/e2e/lib/components_validate.sh`

## Validation workflows

```bash
# Validate component manifests/contracts/fixtures
./test/e2e/run-e2e.sh --validate-components

# Run fast harness contract tests
./test/e2e/tests/run.sh
```

## Notes

- Keep component definitions deterministic and shellcheck-friendly.
- Prefer reusable hooks/scripts under the component directory rather than duplicating logic across components.
- Update `test/e2e/components/STANDARD.md` when the component contract changes.
