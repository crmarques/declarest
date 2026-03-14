# Metadata Overview

Metadata is the translation layer between your logical path model and the real API behavior.

If your API is perfect REST, you may only need a little metadata.
If your API is inconsistent, nested, RPC-ish, or mixed-type, metadata becomes the core modeling tool.

## What metadata can control

### Resource identity and path mapping

- `resource.id`
- `resource.alias`
- `resource.remoteCollectionPath`
- `resource.secretAttributes`
- `resource.externalizedAttributes`

`resource.id` and `resource.alias` accept either full identity templates such as `{% raw %}{{/clientId}}{% endraw %}` or raw JSON Pointer shorthand such as `/clientId`.
If you omit them, DeclaREST defaults identity resolution to `/id`.

### Operation behavior (`operations.*`)

For each operation (`get`, `create`, `update`, `delete`, `list`, `compare`) you can control:

- `path`
- `method`
- `query`
- `headers`
- `body`
- ordered payload mutation pipeline (`transforms`)

### Compare/diff normalization

Use `compare.transforms` to suppress noisy fields before diffing (timestamps, server-generated versions, etc.).

## Metadata files are overrides, not full schema dumps

You usually store only the overrides you need.
DeclaREST applies defaults and merges matching metadata layers at runtime.

Helpful commands:

```bash
# Effective metadata (merged with defaults)
declarest resource metadata get /corporations/acme

# Minimal override view
declarest resource metadata get /corporations/acme --overrides-only

# Render one operation to inspect the final HTTP request mapping
declarest resource metadata render /corporations/acme update
```

## OpenAPI helps, but metadata is the final authority

If an OpenAPI spec is configured, DeclaREST can infer or improve defaults.
But explicit metadata overrides win.

```bash
declarest resource metadata infer /corporations/acme
declarest resource metadata infer /corporations/acme --apply
```

Use inference as a starting point, then refine manually for edge cases.

## Externalized text attributes

Use `resource.externalizedAttributes` when one or more string fields should live in sibling files instead of inline in `resource.yaml`.

Example metadata:

```yaml
resource:
  externalizedAttributes:
    - path: ["script"]
      file: "script.sh"
    - path: ["sequence", "commands", "*", "script"]
      file: "script.sh"
```

Save flow:

1. {% raw %}`declarest resource save /projects/platform` writes `resource.yaml` with `script: '{{include script.sh}}'`.{% endraw %}
2. The original string content is written to `script.sh` next to `resource.yaml`.

Resulting files:

```yaml
# resource.yaml
script: '{% raw %}{{include script.sh}}{% endraw %}'
```

```bash
# script.sh
echo "hello from sidecar"
```

Apply/diff flow:

1. Repository-backed apply/create/update/diff reads `resource.yaml`.
2. When the configured placeholder is present, DeclaREST loads `script.sh`.
3. The effective payload sent to diff/apply uses the file content, not the placeholder string.

This is useful for shell scripts, policy text, certificates, or other long text blobs that are easier to review as standalone files.

For array-backed fields, use `*` to match each element. The configured `file` acts as the base name and DeclaREST appends the matched wildcard indexes before the extension, so `script.sh` becomes `script-0.sh`, `script-1.sh`, and so on.

## Advanced topics

- [Metadata Overrides](metadata-overrides.md) for precedence and merge behavior
- [Custom Paths](metadata-custom-paths.md) for non-REST endpoints and logical-path indirection
- [Advanced Metadata Configuration workflow](../workflows/advanced-metadata-configuration.md) for real Keycloak examples
