# Configuration reference

DeclaREST uses context files (YAML or JSON) to define how it connects to repositories and managed servers.
Use `declarest config add` to register them; omit the config path to run the interactive flow.

## Context file structure

```yaml
repository:
  resource_format: json # json (default) or yaml
  # filesystem or git config
managed_server:
  http:
    base_url: https://example.com/api
    openapi: /path/to/openapi.yaml
secret_store:
  file:
    path: /path/to/secrets.json
    passphrase: change-me
```

## Repository configuration

`repository.resource_format` controls how resource payload files are stored in the repo. Use `json` (default)
for `resource.json` files or `yaml` for `resource.yaml`.

### Filesystem

```yaml
repository:
  resource_format: json
  filesystem:
    base_dir: /path/to/repo
```

### Git local

```yaml
repository:
  resource_format: json
  git:
    local:
      base_dir: /path/to/repo
```

### Git remote

```yaml
repository:
  resource_format: json
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
    openapi: /path/to/openapi.yaml
    default_headers:
      Accept: application/json
    auth:
      bearer_token:
        token: YOUR_TOKEN
    tls:
      ca_cert_file: /path/to/ca.pem
      client_cert_file: /path/to/client.pem
      client_key_file: /path/to/client-key.pem
      insecure_skip_verify: false
```

Auth options for `managed_server.http.auth`:

- `oauth2`: token_url, grant_type, client_id, client_secret, username, password, scope, audience
- `basic_auth`: username, password
- `bearer_token`: token
- `custom_header`: header, token

When `managed_server.http.openapi` is set, DeclaREST loads the OpenAPI spec (URL or file)
to pick smarter default HTTP methods and content types when metadata does not override them.

`managed_server.http.tls.ca_cert_file` lets you verify the server with a custom CA trust bundle.
Provide both `client_cert_file` and `client_key_file` to enable mutual TLS.

## Secret store configuration

File-backed secret store:

```yaml
secret_store:
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
- If you configure `secret_store.file.key`, it must be base64 encoded and decode to exactly 32 bytes (AES-256 requires a 256-bit key); otherwise `declarest secret init` fails. Generate a compliant key with:
  - ``python - <<'PY'
    import os, base64
    print(base64.b64encode(os.urandom(32)).decode())
    PY``
  - `openssl rand -base64 32`
- When using a passphrase, keys are derived using Argon2id by default.
- Secrets files are stored with restrictive permissions.

Vault-backed secret store (KV v1/v2):

```yaml
secret_store:
  vault:
    address: https://vault.example.com
    mount: secret
    path_prefix: declarest
    kv_version: 2
    auth:
      token: s.xxxx
      # password:
      #   username: vault-user
      #   password: vault-pass
      #   mount: userpass
      # approle:
      #   role_id: role-id
      #   secret_id: secret-id
      #   mount: approle
    tls:
      ca_cert_file: /path/to/ca.pem
      client_cert_file: /path/to/client.pem
      client_key_file: /path/to/client-key.pem
      insecure_skip_verify: false
```

Notes:

- `kv_version` defaults to 2; set to 1 to use KV v1 endpoints.
- `mount` defaults to `secret` when omitted.
- `path_prefix` scopes secrets within the mount.
- Provide exactly one auth method: `token`, `password`, or `approle`.
- mTLS is optional; it is enabled when client cert/key files are provided (CA is optional if the server cert is already trusted).

## Metadata configuration

```yaml
metadata:
  base_dir: /path/to/metadata
```

- `metadata.base_dir` optionally points metadata reads/writes to a dedicated directory; leave it unset to keep metadata under the same directory configured for `repository.filesystem.base_dir` or `repository.git.local.base_dir`.

## Context store location

Contexts are stored in `DECLAREST_HOME/.declarest/config` by default (`DECLAREST_HOME` falls back to `$HOME`).

Use `declarest config list` and `declarest config use` to manage the active context.

## Environment overrides

DeclaREST honors these environment variables when determining where the context store lives:

- `DECLAREST_HOME`: override the base home directory that defaults to `$HOME`; changing this adjusts what `DECLAREST_CONFIG_DIR` and `DECLAREST_CONFIG_FILE` resolve to.
- `DECLAREST_CONFIG_DIR`: override the directory that contains the config file (defaults to `DECLAREST_HOME/.declarest`).
- `DECLAREST_CONFIG_FILE`: override the full config file path (defaults to `DECLAREST_CONFIG_DIR/config`).

### Context configuration overrides

Set `DECLAREST_CTX_NAME` to use a specific context instead of the one currently marked as default; if the named context exists, the CLI merges the values defined in the config store with additional `DECLAREST_CTX_<attribute>` overrides (variables take precedence). When the named context does not exist, DeclaREST builds a context purely from the provided `DECLAREST_CTX_<attribute>` values and fails if any required attribute is missing.

Each attribute path in a context definition can be overridden via `DECLAREST_CTX_<PATH>`, where `<PATH>` is the attribute path upper-cased and dots replaced with underscores (e.g., `repository.git.local.base_dir` becomes `DECLAREST_CTX_REPOSITORY_GIT_LOCAL_BASE_DIR`). The values are parsed as strings, booleans (`true`/`false`), integers, or YAML maps (for default headers). Use the tables below to find the env var that matches the attribute you wish to override.

#### Managed server HTTP attributes

| Attribute | Environment variable |
| --- | --- |
| `managed_server.http.base_url` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_BASE_URL` |
| `managed_server.http.openapi` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_OPENAPI` |
| `managed_server.http.default_headers` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_DEFAULT_HEADERS` |
| `managed_server.http.auth.oauth2.token_url` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_TOKEN_URL` |
| `managed_server.http.auth.oauth2.grant_type` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_GRANT_TYPE` |
| `managed_server.http.auth.oauth2.client_id` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_CLIENT_ID` |
| `managed_server.http.auth.oauth2.client_secret` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_CLIENT_SECRET` |
| `managed_server.http.auth.oauth2.username` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_USERNAME` |
| `managed_server.http.auth.oauth2.password` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_PASSWORD` |
| `managed_server.http.auth.oauth2.scope` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_SCOPE` |
| `managed_server.http.auth.oauth2.audience` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_OAUTH2_AUDIENCE` |
| `managed_server.http.auth.custom_header.header` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_CUSTOM_HEADER_HEADER` |
| `managed_server.http.auth.custom_header.token` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_CUSTOM_HEADER_TOKEN` |
| `managed_server.http.auth.basic_auth.username` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_BASIC_AUTH_USERNAME` |
| `managed_server.http.auth.basic_auth.password` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_BASIC_AUTH_PASSWORD` |
| `managed_server.http.auth.bearer_token.token` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_AUTH_BEARER_TOKEN_TOKEN` |
| `managed_server.http.tls.ca_cert_file` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_TLS_CA_CERT_FILE` |
| `managed_server.http.tls.client_cert_file` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_TLS_CLIENT_CERT_FILE` |
| `managed_server.http.tls.client_key_file` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_TLS_CLIENT_KEY_FILE` |
| `managed_server.http.tls.insecure_skip_verify` | `DECLAREST_CTX_MANAGED_SERVER_HTTP_TLS_INSECURE_SKIP_VERIFY` |

#### Repository attributes

| Attribute | Environment variable |
| --- | --- |
| `repository.resource_format` | `DECLAREST_CTX_REPOSITORY_RESOURCE_FORMAT` |
| `repository.git.local.base_dir` | `DECLAREST_CTX_REPOSITORY_GIT_LOCAL_BASE_DIR` |
| `repository.git.remote.url` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_URL` |
| `repository.git.remote.branch` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_BRANCH` |
| `repository.git.remote.provider` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_PROVIDER` |
| `repository.git.remote.auto_sync` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_AUTO_SYNC` |
| `repository.git.remote.auth.basic_auth.username` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_AUTH_BASIC_AUTH_USERNAME` |
| `repository.git.remote.auth.basic_auth.password` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_AUTH_BASIC_AUTH_PASSWORD` |
| `repository.git.remote.auth.ssh.user` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_AUTH_SSH_USER` |
| `repository.git.remote.auth.ssh.private_key_file` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_AUTH_SSH_PRIVATE_KEY_FILE` |
| `repository.git.remote.auth.ssh.passphrase` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_AUTH_SSH_PASSPHRASE` |
| `repository.git.remote.auth.ssh.known_hosts_file` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_AUTH_SSH_KNOWN_HOSTS_FILE` |
| `repository.git.remote.auth.ssh.insecure_ignore_host_key` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_AUTH_SSH_INSECURE_IGNORE_HOST_KEY` |
| `repository.git.remote.auth.access_key.token` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_AUTH_ACCESS_KEY_TOKEN` |
| `repository.git.remote.tls.insecure_skip_verify` | `DECLAREST_CTX_REPOSITORY_GIT_REMOTE_TLS_INSECURE_SKIP_VERIFY` |
| `repository.filesystem.base_dir` | `DECLAREST_CTX_REPOSITORY_FILESYSTEM_BASE_DIR` |

#### Metadata attributes

| Attribute | Environment variable |
| --- | --- |
| `metadata.base_dir` | `DECLAREST_CTX_METADATA_BASE_DIR` |

#### Secret store (file) attributes

| Attribute | Environment variable |
| --- | --- |
| `secret_store.file.path` | `DECLAREST_CTX_SECRET_STORE_FILE_PATH` |
| `secret_store.file.key` | `DECLAREST_CTX_SECRET_STORE_FILE_KEY` |
| `secret_store.file.key_file` | `DECLAREST_CTX_SECRET_STORE_FILE_KEY_FILE` |
| `secret_store.file.passphrase` | `DECLAREST_CTX_SECRET_STORE_FILE_PASSPHRASE` |
| `secret_store.file.passphrase_file` | `DECLAREST_CTX_SECRET_STORE_FILE_PASSPHRASE_FILE` |
| `secret_store.file.kdf.time` | `DECLAREST_CTX_SECRET_STORE_FILE_KDF_TIME` |
| `secret_store.file.kdf.memory` | `DECLAREST_CTX_SECRET_STORE_FILE_KDF_MEMORY` |
| `secret_store.file.kdf.threads` | `DECLAREST_CTX_SECRET_STORE_FILE_KDF_THREADS` |

#### Secret store (Vault) attributes

| Attribute | Environment variable |
| --- | --- |
| `secret_store.vault.address` | `DECLAREST_CTX_SECRET_STORE_VAULT_ADDRESS` |
| `secret_store.vault.mount` | `DECLAREST_CTX_SECRET_STORE_VAULT_MOUNT` |
| `secret_store.vault.path_prefix` | `DECLAREST_CTX_SECRET_STORE_VAULT_PATH_PREFIX` |
| `secret_store.vault.kv_version` | `DECLAREST_CTX_SECRET_STORE_VAULT_KV_VERSION` |
| `secret_store.vault.auth.token` | `DECLAREST_CTX_SECRET_STORE_VAULT_AUTH_TOKEN` |
| `secret_store.vault.auth.password.username` | `DECLAREST_CTX_SECRET_STORE_VAULT_AUTH_PASSWORD_USERNAME` |
| `secret_store.vault.auth.password.password` | `DECLAREST_CTX_SECRET_STORE_VAULT_AUTH_PASSWORD_PASSWORD` |
| `secret_store.vault.auth.password.mount` | `DECLAREST_CTX_SECRET_STORE_VAULT_AUTH_PASSWORD_MOUNT` |
| `secret_store.vault.auth.approle.role_id` | `DECLAREST_CTX_SECRET_STORE_VAULT_AUTH_APPROLE_ROLE_ID` |
| `secret_store.vault.auth.approle.secret_id` | `DECLAREST_CTX_SECRET_STORE_VAULT_AUTH_APPROLE_SECRET_ID` |
| `secret_store.vault.auth.approle.mount` | `DECLAREST_CTX_SECRET_STORE_VAULT_AUTH_APPROLE_MOUNT` |
| `secret_store.vault.tls.ca_cert_file` | `DECLAREST_CTX_SECRET_STORE_VAULT_TLS_CA_CERT_FILE` |
| `secret_store.vault.tls.client_cert_file` | `DECLAREST_CTX_SECRET_STORE_VAULT_TLS_CLIENT_CERT_FILE` |
| `secret_store.vault.tls.client_key_file` | `DECLAREST_CTX_SECRET_STORE_VAULT_TLS_CLIENT_KEY_FILE` |
| `secret_store.vault.tls.insecure_skip_verify` | `DECLAREST_CTX_SECRET_STORE_VAULT_TLS_INSECURE_SKIP_VERIFY` |

Run `declarest config env` to see the resolved values (environment vs default) that DeclaREST is using in the current shell.
