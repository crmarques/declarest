# Repository Layout and Normalization

## Logical path normalization (MUST)
- Must start with `/`.
- Must use `/` as separator.
- Must not contain `..` segments.
- Must not contain empty segments (no `//`).
- Must not escape repo root (repository manager enforces).
- `_` is reserved for generic metadata directories.

## On-disk layout (MUST)
For a resource with logical path `/fruits/apples/apple-01`:

```
/fruits/
  _/metadata.json
  apples/
    _/metadata.json
    apple-01/
      resource.json
      metadata.json
```

## Resource payload file (MUST)
- Filename: `resource.json`.
- JSON, stable/deterministic formatting if rewritten.
- Do not print secrets to stdout (see specs/07-quality-security.md).
