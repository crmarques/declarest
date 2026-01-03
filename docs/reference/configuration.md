# Configuration reference

DeclaREST uses context files (YAML or JSON) to define how it connects to repositories and managed servers.
Use `declarest config add` or `declarest config init` to register them.

## Context file structure

```yaml
repository:
  # filesystem or git config
managed_server:
  http:
    base_url: https://example.com/api
secret_manager:
  file:
    path: /path/to/secrets.json
    passphrase: change-me
```

## Repository configuration

### Filesystem

```yaml
repository:
  filesystem:
    base_dir: /path/to/repo
```

### Git local

```yaml
repository:
  git:
    local:
      base_dir: /path/to/repo
```

### Git remote

```yaml
repository:
  git:
    local:
      base_dir: /path/to/repo
    remote:
      url: https://example.com/org/repo.git
      branch: main
      provider: github
      auto_sync: true
      auth:
        access_key:
          token: YOUR_TOKEN
      tls:
        insecure_skip_verify: false
```

Auth options for `git.remote.auth`:

- `basic_auth`: username/password
- `ssh`: user, private_key_file, passphrase, known_hosts_file, insecure_ignore_host_key
- `access_key`: token

## Managed server configuration

Currently DeclaREST supports HTTP-based servers.

```yaml
managed_server:
  http:
    base_url: https://api.example.com
    default_headers:
      Accept: application/json
    auth:
      bearer_token:
        token: YOUR_TOKEN
    tls:
      insecure_skip_verify: false
```

Auth options for `managed_server.http.auth`:

- `oauth2`: token_url, grant_type, client_id, client_secret, username, password, scope, audience
- `basic_auth`: username, password
- `bearer_token`: token

## Secrets manager configuration

File-backed secrets manager:

```yaml
secret_manager:
  file:
    path: /path/to/secrets.json
    passphrase: change-me
    # Or use a raw key instead of a passphrase
    # key: your-raw-key
    # key_file: /path/to/key
    # passphrase_file: /path/to/passphrase
    # kdf:
    #   time: 3
    #   memory: 65536
    #   threads: 4
```

Notes:

- Provide either `key` or `passphrase` (or their *_file variants).
- When using a passphrase, keys are derived using Argon2id by default.
- Secrets files are stored with restrictive permissions.

## Context store location

Contexts are stored in `~/.declarest/config` by default.
Use `declarest config list` and `declarest config use` to manage the active context.
