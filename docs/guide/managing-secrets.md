# Managing Secrets

DeclaREST keeps sensitive values out of Git repository files by combining metadata declarations, payload placeholders, and a secret store.

## How the model works

1. **Metadata declares** which attributes are secrets (`resource.secretAttributes`).
2. **Payloads store placeholders** instead of plaintext values.
3. **Secret store holds** the actual values (encrypted file or HashiCorp Vault).
4. **Remote workflows resolve** placeholders before sending API requests.

## Declaring secret attributes

Add JSON Pointer paths to `resource.secretAttributes` in metadata:

```json
{
  "resource": {
    "secretAttributes": [
      "/credentials/password",
      "/config/bindCredential/0",
      "/clientSecret"
    ]
  }
}
```

Place declarations at the widest safe metadata scope (collection level) so all child resources inherit them.

## Placeholder syntax

Two forms are supported in resource payload files:

| Placeholder | Key derivation |
|-------------|---------------|
| {% raw %}`{{secret .}}`{% endraw %} | Derived from logical path + attribute path (recommended) |
| {% raw %}`{{secret custom-key}}`{% endraw %} | Uses logical path + custom key suffix |

Example in a resource file:

```yaml
credentials:
  password: "{% raw %}{{secret .}}{% endraw %}"
apiToken: "{% raw %}{{secret service-api-token}}{% endraw %}"
```

Prefer {% raw %}`{{secret .}}`{% endraw %} unless you need a stable custom key name.

## Configuring a secret store

Set up either backend in your context configuration:

**File-backed** (encrypted, default):

```yaml
secretStore:
  file:
    baseDir: /path/to/secrets    # defaults to ~/.declarest/secrets/
    passphrase:
      prompt: true
```

**HashiCorp Vault**:

```yaml
secretStore:
  vault:
    address: https://vault.example.com
    mountPath: secret
    auth:
      token:
        credentialsRef:
          name: vault-token
```

Then initialize the store:

```bash
declarest secret init
```

See [Configuration reference](../reference/configuration.md) for full schema details.

## Safe save workflow

The recommended way to import resources containing secrets:

```bash
declarest resource save /corporations/acme --secret-attributes
```

DeclaREST will:

1. Detect plaintext secret candidates based on metadata declarations.
2. Store values in the secret store.
3. Replace them with {% raw %}`{{secret .}}`{% endraw %} placeholders in the repository file.

Variants:

```bash
# handle only specific attributes
declarest resource save /corporations/acme --secret-attributes /clientSecret,/apiKey

# bypass plaintext guard (local testing only, do not commit)
declarest resource save /corporations/acme --allow-plaintext
```

## Detecting and fixing secrets in existing repositories

Scan for likely plaintext secrets and persist declarations into metadata:

```bash
# scan a subtree
declarest secret detect /customers/

# auto-fix: persist detected attributes into metadata
declarest secret detect --fix /customers/

# fix one specific attribute
declarest secret detect --fix --secret-attribute /clientSecret /customers/

# scan from a payload before saving
cat payload.json | declarest secret detect --fix --path /corporations/acme
```

## Secret store operations

```bash
declarest secret init                                         # initialize store
declarest secret set /corporations/acme /apiToken super-secret # set a value
declarest secret list /corporations/acme                       # list keys for a path
declarest secret list /projects --recursive                    # list recursively
declarest secret get /corporations/acme /apiToken              # retrieve a value
```

## Inspecting secrets

View resolved plaintext only when explicitly needed:

```bash
declarest resource get /corporations/acme --show-secrets
```

Use sparingly -- avoid in logs or shared terminals.

## Troubleshooting

### Save fails with plaintext-secret warning

Possible causes:

- Metadata does not declare the secret attribute yet.
- Secret store is not configured or initialized.
- Some detected secrets were left unhandled.

Fix order:

1. Declare `secretAttributes` in metadata.
2. Ensure secret store works: `secret init`, `secret list`.
3. Retry `resource save --secret-attributes`.

### One-off plaintext import

Use `--allow-plaintext` only for temporary local testing. Do not commit the result.
