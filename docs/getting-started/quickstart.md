# Quickstart

This is a happy-flow walkthrough: one context, one resource, one edit, one apply.

## 1. Create a context

Interactive (recommended for first use):

```bash
declarest config create
```

If you prefer file-based setup:

```bash
declarest config print-template > /tmp/contexts.yaml
# edit /tmp/contexts.yaml
declarest config add --file /tmp/contexts.yaml --set-current
```

Check the active configuration:

```bash
declarest config current
declarest config check
```

## 2. Save one resource from the API into the repository

```bash
declarest resource save /corporations/acme
```

This writes the payload to the repository base dir configured in your context, for example:

- `<repo-base-dir>/corporations/acme/resource.json`
- or `resource.yaml` when `repository.resource-format: yaml`

## 3. Inspect and edit locally

```bash
declarest resource get --source repository /corporations/acme
# edit the file in your editor
```

## 4. Diff and apply

```bash
declarest resource diff /corporations/acme
declarest resource apply /corporations/acme
```

## 5. Verify

```bash
declarest resource get /corporations/acme
```

## What to learn next

- [Paths and Selectors](../concepts/paths-and-selectors.md)
- [Metadata overview](../concepts/metadata.md)
- [Configuration reference](../reference/configuration.md)
- [Advanced Metadata Configuration workflow](../workflows/advanced-metadata-configuration.md)
