# Quality, Error Handling, and Security

## Error handling
- Wrap root causes with `%w`.
- User-facing errors must explain what failed (path, operation, server/repo side).

## Quality gates
- `go test ./...` must pass.
- New logic must include tests or a documented validation note (prefer tests).
- Always run `gofmt -w .` after editing/creating any Go file.

## Security invariants (MUST)
- Never allow path traversal outside the configured repo root.
- Never print credentials/tokens/secrets unless explicitly requested.
- Destructive git actions require explicit user intent.
- HTTP client respects TLS settings defined by context.
- External dependencies must be reputable and maintained.
