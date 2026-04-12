# Working with Resources

> This page covers day-to-day resource operations. Make sure you understand [Core Concepts](core-concepts.md) first.

## Prerequisites

```bash
declarest context current       # confirm active context
declarest context check         # validate context config
declarest server check          # confirm API connectivity
```

## Save resources

Import remote state into the local repository:

```bash
declarest resource save /corporations/acme
```

Variants:

```bash
# overwrite existing local resource
declarest resource save /corporations/acme --force

# detect and mask secrets during save
declarest resource save /corporations/acme --secret-attributes

# save from explicit payload instead of remote read
cat payload.json | declarest resource save /corporations/acme --payload -
```

### Save collections

```bash
# fan out list items into individual resource directories (default)
declarest resource save /customers/

# store collection payload as one file
declarest resource save /customers/ --mode single
```

### Wildcard save for bulk discovery

Use `_` wildcard segments to expand remote collections:

```bash
declarest resource save /admin/realms/_/clients/_
```

This discovers and saves all concrete resources matching the pattern.

## Review local state

```bash
declarest resource get --source repository /corporations/acme
declarest resource list --source repository /customers/
```

## Metadata-backed defaults

When many sibling resources share the same fields, extract shared values into metadata-managed defaults so `resource.<ext>` keeps only explicit overrides:

```bash
# preview what defaults would be inferred
declarest resource defaults infer /corporations/acme

# persist inferred defaults
declarest resource defaults infer /corporations/acme --save

# inspect current defaults
declarest resource defaults get /corporations/acme

# edit defaults
declarest resource defaults edit /corporations/acme

# check if current defaults match what would be inferred today
declarest resource defaults infer /corporations/acme --check
```

### Managed-service probing

Probe server-added defaults (creates/removes temporary resources):

```bash
declarest resource defaults infer /corporations/acme --managed-service --yes
declarest resource defaults infer /corporations/acme --managed-service --wait 2s --yes
```

### Print only explicit overrides

After defaults exist, compact payloads to just the non-default values:

```bash
declarest resource get --source repository /corporations/acme --prune-defaults
declarest resource save /corporations/acme --prune-defaults --force
```

## Diff and apply

### Compare desired vs real state

```bash
declarest resource diff /corporations/acme
declarest resource diff /corporations --recursive
declarest resource diff /corporations --recursive --list    # paths only
declarest resource diff /corporations/acme --color always
```

Use `--list` for drifting paths only, or `-o json|yaml` for structured output.

### Apply desired state to the API

```bash
declarest resource apply /corporations/acme
declarest resource apply /corporations/acme --refresh     # re-read after write
declarest resource apply /customers/ --recursive
```

## Edit, copy, and delete

```bash
# open resource in editor
declarest resource edit /corporations/acme

# copy a resource
declarest resource copy /corporations/acme /corporations/acme-copy

# delete from remote API
declarest resource delete /corporations/acme
```

## Create and update

```bash
# create a new resource on the remote API from local state
declarest resource create /corporations/acme

# update an existing resource
declarest resource update /corporations/acme

# recursive operations
declarest resource create /customers/ --recursive
declarest resource update /customers/ --recursive
```

## Inspect before you mutate

Use these commands to understand what DeclaREST will do before making changes.

### Confirm target path

```bash
declarest resource list --source repository /customers/
declarest resource list --source remote-server /customers/
declarest resource list --source remote-server /customers/ --output text
```

### Inspect metadata

```bash
# effective metadata (defaults + merged overrides)
declarest resource metadata get /corporations/acme

# only authored overrides
declarest resource metadata get /corporations/acme --overrides-only
```

### Render operation specs

See the exact HTTP request that would be sent:

```bash
declarest resource metadata render /corporations/acme get
declarest resource metadata render /corporations/acme update
declarest resource metadata render /customers/ list
```

### Explain the reconciliation plan

```bash
declarest resource explain /corporations/acme
```

### Inspect payload with metadata context

```bash
declarest resource get /corporations/acme --show-metadata
```

### Check server connectivity

```bash
declarest server check
declarest server get base-url
declarest server get token-url
declarest server get access-token    # OAuth2 only
```

### Enable debug output

```bash
declarest -d resource get /corporations/acme
```

## Common diagnosis checklist

| Symptom | Check |
|---------|-------|
| Wrong endpoint | `resource.remoteCollectionPath` and operation `path` |
| Wrong ID/alias in URL | `resource.id` and `resource.alias` |
| List returns extra objects | add `list.payload.jqExpression` filter |
| Diff shows noise | tune `compare` suppress/filter rules |
| Write payload rejected | inspect `create`/`update` payload transforms |
| Command fails silently | run with `-d` for debug output |
