# Repository Providers

Repository provider-specific helpers should live here (fs, git, gitlab, gitea, github).
Managed server harnesses can source these scripts to keep repo logic consistent.
Each provider must expose a `component.sh` contract; see `docs/e2e-components.md`.
