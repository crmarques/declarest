# Syntax (paths, selectors, placeholders)

## Logical paths

Use logical absolute paths in CLI commands.

Examples:

- resource path: `/customers/acme`
- collection path: `/customers/`

Rules:

- must start with `/`
- `/` separates segments
- trailing `/` means collection
- `_` is reserved for selector/wildcard semantics

## Collection trailing slash rules

- `/customers/acme` targets one resource
- `/customers/` targets a collection

This affects command behavior (for example `resource list`, metadata operation defaults, collection metadata placement).

## Selector paths (`_` segments)

Selectors are metadata patterns, not concrete resources.

Example selector:

- `/admin/realms/_/clients/_/`

This means metadata rules can apply to any realm/client path matching that structure.

## Metadata file path syntax

- Resource metadata: `<logical-path>/metadata.json` (or `.yaml`)
- Collection/subtree metadata: `<collection-path>/_/metadata.json` (or `.yaml`)
- Resource payload: `<logical-path>/resource.<ext>`

## Secret placeholder syntax

Use placeholders in payload files, not plaintext secrets.

Supported forms:

- {% raw %}`{{secret .}}`{% endraw %}
- {% raw %}`{{secret custom-key}}`{% endraw %}

Behavior summary:

- {% raw %}`{{secret .}}`{% endraw %}: key is derived from logical path + attribute path.
- {% raw %}`{{secret custom-key}}`{% endraw %}: key uses logical path + custom key suffix.

Examples:

```yaml
credentials:
  password: "{% raw %}{{secret .}}{% endraw %}"
apiToken: "{% raw %}{{secret service-api-token}}{% endraw %}"
```

See [Secrets](../concepts/secrets.md) for end-to-end workflows.
