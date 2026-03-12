# Resource Files

A resource is a logical object stored in the repository as `resource.<ext>`, using the trusted media type from the managed-server response or explicit payload input. When shared values make the resource file noisy, you can also keep them in an optional sibling `defaults.<ext>` sidecar.

## File naming

The main payload filename is always `resource.<ext>`. When defaults sidecars are supported for that payload type, the companion filename is `defaults.<ext>`.

Examples:

- `application/json` -> `resource.json`
- `application/yaml` -> `resource.yaml`
- `application/xml` -> `resource.xml`
- opaque input file `private.key` -> `resource.key` with internal media type `binary`
- unknown payload with no usable suffix hint -> `resource.bin` with internal media type `binary`

## Repository layout

Examples for logical path `/corporations/acme`:

- payload: `corporations/acme/resource.json` (or another `resource.<ext>`)
- defaults sidecar (optional): `corporations/acme/defaults.json`
- resource-only metadata (optional): `corporations/acme/metadata.yaml`

Collection metadata for `/customers/`:

- `customers/_/metadata.yaml`

## Defaults sidecars

Use a defaults sidecar when many resources in the same collection share the same object fields and you want `resource.<ext>` to keep only explicit overrides. DeclaREST reads the effective desired state as:

- `defaults.<ext>` merged with `resource.<ext>`
- object keys merge recursively
- arrays are replaced as a whole
- explicit values in `resource.<ext>` override the defaults, including `null`

Current defaults-sidecar support is intentionally narrow: use it with merge-capable object payloads such as `json`, `yaml`, `ini`, and `properties`. Opaque or non-merge-friendly formats stay single-file resources for now.

Typical workflow:

```bash
declarest resource defaults infer /corporations/acme
declarest resource defaults infer /corporations/acme --save
declarest resource defaults get /corporations/acme
declarest resource get --source repository /corporations/acme --prune-defaults
```

That lets the repository keep shared defaults in one sidecar while normal repository-backed reads, diffs, and apply flows still use the merged effective resource.

## Resource payloads vs collection payloads

DeclaREST can persist collection responses in three modes with `resource save`:

- `--mode auto` (default): non-list payloads save as one resource; list payloads fan out into one resource directory per item
- `--mode items`: each item becomes its own resource directory
- `--mode single`: stores the collection payload as one resource file

Use `--mode auto` or `--mode items` for normal GitOps/reconciliation. Use `--mode single` for opaque list endpoints or snapshots.

## Includes inside resource payloads

Resource files support the literal include directive:

- {% raw %}`{{include file.ext}}`{% endraw %}

The included file path is resolved relative to the current resource directory.
Structured JSON/YAML content is merged as data; non-JSON/YAML content is included as text.

Example:

```yaml
service:
  config: "{% raw %}{{include config.json}}{% endraw %}"
  script: "{% raw %}{{include deploy.sh}}{% endraw %}"
```

This is useful when a resource embeds long scripts, policy documents, or nested config fragments.

## Secrets in resource files

Sensitive attributes should be stored as placeholders, not plaintext:

- {% raw %}`{{secret .}}`{% endraw %}
- {% raw %}`{{secret custom-key}}`{% endraw %}

The secret values live in the configured secret store, and the placeholder stays in Git.
See [Secrets](secrets.md).

## Inspecting resource behavior before apply

Use these commands together:

```bash
declarest resource get --source repository /corporations/acme
declarest metadata get /corporations/acme
declarest metadata render /corporations/acme update
```

That combination lets you inspect payload + metadata + rendered remote request mapping before a write.
