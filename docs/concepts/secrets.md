# Secrets

DeclaREST keeps sensitive values out of repository payload files by using metadata-declared secret attributes plus placeholders.

## How the model works

1. Metadata declares which attributes are secrets (`resource.secretAttributes`).
2. Resource payload files store placeholders instead of plaintext.
3. Secret values are stored in a configured secret store (file or Vault).
4. Remote workflows resolve placeholders before sending requests.

## Declaring secret attributes

Example metadata:

```json
{
  "resource": {
    "secretAttributes": [
      "/credentials/password",
      "/config/bindCredential/0"
    ]
  }
}
```

Attribute paths use JSON Pointer syntax with `/`-separated tokens.

## Placeholders in resource files

Supported forms:

- {% raw %}`{{secret .}}`{% endraw %} -> uses the attribute path as the key suffix
- {% raw %}`{{secret custom-key}}`{% endraw %} -> uses a custom key suffix

Secrets are scoped by logical path, so the effective key is path-aware.

## Safe save flow (recommended)

When importing from a remote API, let DeclaREST detect/store/mask secrets during save:

```bash
declarest resource save /corporations/acme --secret-attributes
```

Options:

- `--secret-attributes` (all detected secret candidates)
- `--secret-attributes /attr1,/attr2` (selected attributes)
- `--allow-plaintext` (bypass plaintext-secret guard; use carefully)

## Detecting and fixing secret metadata on existing repositories

```bash
# scan a subtree for likely secrets
declarest secret detect /customers/

# persist detected attributes into metadata
declarest secret detect --fix /customers/
```

You can also scan an input payload before saving:

```bash
cat payload.json | declarest secret detect --fix --path /corporations/acme
```

## Secret store operations

```bash
declarest secret init
declarest secret set /corporations/acme /apiToken super-secret
declarest secret list /corporations/acme
declarest secret list /projects --recursive
declarest secret get /corporations/acme /apiToken
```

## Backends

DeclaREST supports:

- file-backed encrypted secret store
- HashiCorp Vault (KV)

See [Configuration reference](../reference/configuration.md) for setup details.

## Operational tips

- Keep `secretAttributes` defined at the widest safe metadata scope.
- Prefer {% raw %}`{{secret .}}`{% endraw %} unless you need a stable custom key name.
- Use `resource get --show-secrets` only when you explicitly need plaintext output.
- Do not commit plaintext secrets with `--allow-plaintext` except for throwaway local testing.
