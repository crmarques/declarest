# Secrets and Safe Save

This page shows a safe workflow for importing resources that contain sensitive values.

## Goal

Save resources into Git without committing plaintext credentials.

## 1. Configure a secret store

Set up either `secretStore.file` or `secretStore.vault` in your context.
Then initialize it (when required by the backend):

```bash
declarest secret init
```

## 2. Declare secret attributes in metadata

Add `resource.secretAttributes` at the correct collection scope.

Example:

```json
{
  "resource": {
    "secretAttributes": ["/credentials/password", "/clientSecret"]
  }
}
```

## 3. Import remote resources safely

```bash
declarest resource save /corporations/acme --secret-attributes
```

DeclaREST will:

- detect plaintext secret candidates
- store handled values in the secret store
- replace them with {% raw %}`{{secret .}}`{% endraw %} placeholders in repository payloads
- keep metadata secret declarations aligned

## 4. Verify what was stored

```bash
declarest resource get --source repository /corporations/acme
declarest secret list /corporations/acme
declarest secret get /corporations/acme /clientSecret
```

## 5. Detect and fix secret metadata across existing repos

```bash
declarest secret detect /customers/
declarest secret detect --fix /customers/
```

Apply one attribute only:

```bash
declarest secret detect --fix --secret-attribute /clientSecret /customers/
```

## 6. Inspect plaintext only when necessary

```bash
declarest resource get /corporations/acme --show-secrets
```

Use this sparingly (local terminal only, avoid logs).

## Troubleshooting

### Save fails with plaintext-secret warning

Possible reasons:

- metadata does not declare the secret attribute yet
- secret store is not configured
- you passed `--secret-attributes` but left some detected secrets unhandled

Fix order:

1. declare `secretAttributes`
2. ensure secret store works (`secret init`, `secret list`, `secret get`)
3. retry `resource save --secret-attributes`

### I need to import a one-off fixture with plaintext

Use `--allow-plaintext` only for temporary/local testing and avoid committing the result.
