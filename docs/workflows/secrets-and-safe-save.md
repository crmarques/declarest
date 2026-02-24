# Secrets and Safe Save

This page shows a safe workflow for importing resources that contain sensitive values.

## Goal

Save resources into Git without committing plaintext credentials.

## 1. Configure a secret store

Set up either `secret-store.file` or `secret-store.vault` in your context.
Then initialize it (when required by the backend):

```bash
declarest secret init
```

## 2. Declare secret attributes in metadata

Add `resourceInfo.secretInAttributes` at the correct collection scope.

Example:

```json
{
  "resourceInfo": {
    "secretInAttributes": ["credentials.password", "clientSecret"]
  }
}
```

## 3. Import remote resources safely

```bash
declarest resource save /corporations/acme --handle-secrets
```

DeclaREST will:

- detect plaintext secret candidates
- store handled values in the secret store
- replace them with `{{secret .}}` placeholders in repository payloads
- keep metadata secret declarations aligned

## 4. Verify what was stored

```bash
declarest resource get --source repository /corporations/acme
declarest secret get /corporations/acme
```

## 5. Detect and fix secret metadata across existing repos

```bash
declarest secret detect /customers/
declarest secret detect --fix /customers/
```

Apply one attribute only:

```bash
declarest secret detect --fix --secret-attribute clientSecret /customers/
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
- you passed `--handle-secrets` but left some detected secrets unhandled

Fix order:

1. declare `secretInAttributes`
2. ensure secret store works (`secret init`, `secret get`)
3. retry `resource save --handle-secrets`

### I need to import a one-off fixture with plaintext

Use `--ignore` only for temporary/local testing and avoid committing the result.
