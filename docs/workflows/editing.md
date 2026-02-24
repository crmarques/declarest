# Editing Metadata and Contexts

This page shows safe edit loops for the two things that most often change during onboarding an API: context configuration and metadata rules.

## Editing contexts safely

### Inspect current context

```bash
declarest config current
declarest config show
declarest config resolve
```

### Test overrides without changing stored config

Use `config resolve --set` to preview runtime overrides.

```bash
declarest config resolve \
  --set resource-server.http.base-url=https://staging-api.example.com \
  --set repository.resource-format=yaml
```

This is the safest way to test environment-specific changes before editing the stored context.

### Validate config files before import/update

```bash
declarest config print-template > /tmp/contexts.yaml
# edit /tmp/contexts.yaml

declarest config validate --payload /tmp/contexts.yaml
declarest config add --file /tmp/contexts.yaml --set-current
```

Update an existing context catalog entry:

```bash
declarest config update --payload /tmp/contexts.yaml
```

## Editing metadata safely

### Recommended loop

1. Inspect effective metadata
2. Save a small override change
3. Render the concrete operation(s)
4. Run `resource explain`
5. Only then run `save/apply/update/delete`

### Inspect first

```bash
declarest metadata get /corporations/acme
declarest metadata get /corporations/acme --overrides-only
```

### Write metadata from a file or stdin

```bash
declarest metadata set /customers/ --payload customers-metadata.json

# or
cat customers-metadata.json | declarest metadata set /customers/ --payload -
```

Remove metadata for a path when refactoring selector layout:

```bash
declarest metadata unset /customers/
```

### Validate rendered operations after every metadata change

```bash
declarest metadata render /corporations/acme get
declarest metadata render /corporations/acme create
declarest metadata render /corporations/acme update
declarest metadata render /customers/ list

declarest resource explain /corporations/acme
```

### Use inference as a baseline, then customize

```bash
declarest metadata infer /customers/
declarest metadata infer /customers/ --apply
```

Inference is a starting point. Advanced APIs almost always need manual overrides afterward.

See [Advanced Metadata Configuration](advanced-metadata-configuration.md).
