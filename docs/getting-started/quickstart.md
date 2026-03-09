# Quickstart

This is a happy-flow walkthrough: one context, one resource, one edit, one apply.

## 1. Create a context

A context tells DeclaREST where your repository lives and how to reach the managed server.

Interactive (recommended for first use):

```bash
declarest context add
```

If you prefer file-based setup:

```bash
declarest context print-template > /tmp/contexts.yaml
# edit /tmp/contexts.yaml
declarest context add --payload /tmp/contexts.yaml --set-current
```

Check the active configuration:

```bash
declarest context current
declarest context check
```

## 2. Save one resource from the API into the repository

Pull the current remote state so you have a baseline to work from:

```bash
declarest resource save /corporations/acme
```

This writes the payload to the repository base dir configured in your context, for example:

- `<repo-base-dir>/corporations/acme/resource.json`
- or another `resource.<ext>` when the managed server responds with a different media type

## 3. Inspect and edit locally

Review and modify the saved resource to define your desired state:

```bash
declarest resource get --source repository /corporations/acme
# edit the file in your editor
```

## 4. Diff and apply

Compare your local changes against the live server, then push them:

```bash
declarest resource diff /corporations/acme
declarest resource apply /corporations/acme
```

## 5. Verify

Confirm the remote state matches what you applied:

```bash
declarest resource get /corporations/acme
```

## What to learn next

- [Contexts](../concepts/context.md) — understand how contexts connect everything
- [Paths and Selectors](../concepts/paths-and-selectors.md) — learn the path model
- [Metadata overview](../concepts/metadata.md) — adapt DeclaREST to non-REST APIs
- [Configuration reference](../reference/configuration.md) — all context options
