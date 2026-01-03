# Secrets

DeclaREST keeps secrets out of the repository by storing them in a dedicated secrets manager.
Resources reference secrets using placeholders.

At a high level:

1. Metadata marks which JSON attributes are secret.
2. Resource files store placeholders instead of plaintext values.
3. The secrets manager stores the real values, keyed by `<logical-path> + secret key`.

## How secrets are marked

Add secret paths in metadata using `resourceInfo.secretInAttributes`.
Paths use dot notation and optional list indexes, for example:

- `secret`
- `config.bindCredential[0]`

## Placeholders in resource files

When secrets are stored, values are replaced with placeholders in `resource.json`:

- `{{secret .}}` uses the default key derived from the attribute path.
- `{{secret "custom"}}` uses a custom key name.

## How secret keys work

- Placeholders with `.` use the attribute path as the secret key (for example `credentials.password`).
- Placeholders with a string use that string as the secret key (for example `{{secret "db_password"}}`).
- Secret keys are stored per-resource: the same key name under two different resource paths are different secrets.

## Example

If metadata marks an attribute as secret:

```json
{
  "resourceInfo": {
    "secretInAttributes": ["credentials.password"]
  }
}
```

Then `resource.json` stores a placeholder instead of the plaintext password:

```json
{
  "credentials": {
    "password": "{{secret .}}"
  }
}
```

## Common workflow

```bash
# Initialize the secrets manager
declarest secret init

# Scan resources and map secrets into metadata
declarest secret check --fix

# Store a secret value (key is per-resource)
declarest secret add /notes/n-1001 credentials.password change-me
```

## Printing resources with or without secrets

- Default output masks secrets.
- Use `--with-secrets` to resolve and print secret values.
- Saving secrets into the repository requires `--force`.

When you run `declarest resource get --save` without `--with-secrets`, DeclaREST stores secrets in the secrets manager (when configured) and saves placeholders in the repository.

## Secrets manager

The built-in file secrets manager stores encrypted secrets in a JSON file.
It requires either a raw key or a passphrase. See [Configuration](../reference/configuration.md).
