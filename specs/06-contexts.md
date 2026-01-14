# Contexts and Configuration

## Context commands
- `config add` (interactive when no config path is provided)
- `config update`, `config delete`, `config use`, `config rename`
- `config list`, `config current`, `config check`, `config print-template`

## Reconciler
Commands obtain a reconciler via `loadDefaultReconciler` (or equivalent).

## Repository configuration highlights
- `repository.git.local.base_dir`: repo root.
- `repository.git.remote.url` and `repository.git.remote.branch`: override remote endpoint/branch.
- `repository.git.remote.provider`: `github` or `gitlab` to tune token auth defaults.
- `repository.git.remote.auto_sync`: automatic pushes after local changes (default true).
- `repository.git.remote.auth`: `basic_auth`, `access_key`, or `ssh` (user, private_key_file, passphrase, known_hosts_file, insecure_ignore_host_key).
- `repository.git.remote.tls.insecure_skip_verify`: TLS verification for Git over HTTPS.
- `repository.filesystem.base_dir`: non-Git filesystem repository.
- `metadata.base_dir`: optional path where metadata files are stored; defaults to the same base directory configured for the repository.

The context store also captures a top-level `defaultEditor` entry (alongside `contexts` and `currentContext`). When present, `config edit` and `metadata edit` use the configured command to launch your editor; otherwise they default to `vi`. `--editor` overrides both the store value and the fallback.

If an existing context format already exists, do not invent a new one.
