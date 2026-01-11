# Ad-hoc requests

When you need to issue a one-off HTTP call without mutating the repository, `declarest ad-hoc` lets you send the managed server a request while still honoring the metadata rules (headers, query parameters, payload transformations, and templated placeholders).

## When to reach for ad-hoc

- Troubleshoot a collection or resource before you synchronize it with Git.
- Validate metadata-supplied headers/paths without changing local files.
- Run exploratory requests (POST/PUT/PATCH/DELETE) with the same metadata and header merging that a `resource` command would use.

## Sending a request

Use `declarest ad-hoc <method>` (available methods are `get`, `post`, `put`, `patch`, `delete`) and point it at the logical path either positionally or with `--path`:

```
declarest ad-hoc get /teams/platform/users/alice
```

DeclaREST loads the metadata/operation for that path, merges headers/query parameters, and formats the response body like other resource commands.

### Headers and payload

- Add metadata-aware default headers back with `--default-headers` (Accept/Content-Type re-applied even when metadata clears them).
- Override or add headers with `--header "Name: value"` or `--header "Name=value"` (the command accepts both separators).
- Inline a request body with `--payload '{"key":"value"}'` or reference a file `--payload @payload.json`. The CLI reports whether the payload came from a file.

### Output and status

JSON responses are pretty-printed to stdout. By default a `[OK] METHOD PATH STATUS` summary is emitted to stderr; pass `--no-status` to leave only the payload on stdout when piping or scripting.

### Example

```
declarest ad-hoc post /teams/platform/users \
  --payload @resources/new-user.json \
  --header "X-Request-ID=$(uuidgen)" \
  --default-headers
```

The server sees the same metadata-derived headers as a `resource create` would, plus your overrides, so you can confidently exercise any endpoint before committing changes.
