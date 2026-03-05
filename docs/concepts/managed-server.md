# Managed Server

A `ManagedServer` defines how DeclaREST talks to a target API.

It represents the real system that should converge to Git desired state.

## What it contains

`ManagedServer.spec` includes:

- `http.baseURL`
- `http.auth` (exactly one of `oauth2`, `basicAuth`, `customHeaders`)
- optional `http.tls`, `http.proxy`, `http.requestThrottling`
- optional `openapi` and `metadata` artifacts
- optional `pollInterval`

## Auth and TLS

Auth and TLS are part of connectivity, not desired resource content.

- secrets are referenced from Kubernetes `Secret` objects
- auth mode is explicit and validated
- TLS settings are validated before reconcile use

## Relationship with `SyncPolicy`

`SyncPolicy` does not redefine connection details. It references one `ManagedServer`:

- `spec.managedServerRef.name`

That means multiple policies can target the same server with different source paths, as long as source scopes do not overlap.

## Practical guidance

- Keep one `ManagedServer` per endpoint/auth profile.
- Start with minimal auth setup, then add proxy/TLS/throttling only when needed.
- Validate connectivity first before debugging metadata or sync behavior.
