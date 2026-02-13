# Test Scripts

Shared test helpers live here as the harness grows. Managed server scripts should
prefer sourcing reusable helpers from this directory instead of duplicating logic.
The component registry used by `tests/run-tests.sh` lives in `components.sh`.
