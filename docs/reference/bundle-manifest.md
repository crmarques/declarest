# Metadata Bundle Manifest

`bundle.yaml` is the canonical manifest consumed when a context points at a metadata bundle through `metadata.bundle` (shorthand reference) or `metadata.bundleFile` (local archive). The manifest is decoded strictly: any unknown key fails with a validation error.

## Required fields

```yaml
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.0.0           # semver-2; leading "v" is normalized away
description: Metadata bundle for Keycloak Admin REST API.
declarest:
  metadataRoot: metadata # repository-relative path
```

## Optional fields

```yaml
deprecated: false
declarest:
  openapi: openapi.yaml                # repository-relative path or http(s) URL
  compatibleDeclarest: ">=0.1.0"       # Masterminds/semver constraint
  compatibleManagedService:
    product: keycloak                  # ^[a-z0-9][a-z0-9-]*$
    versions: ">=26.0.0 <27.0.0"       # Masterminds/semver constraint
distribution:
  artifactTemplate: keycloak-bundle-{version}.tar.gz  # MUST equal "<name>-{version}.tar.gz"
```

Anything else (`home`, `sources`, `keywords`, `license`, `maintainers`, `annotations`, `declarest.shorthand`, `declarest.metadataFileName`, `distribution.repo`, `distribution.tagTemplate`, ...) is **not** part of the consumer contract and MUST be omitted; the strict decoder rejects unknown keys.

## Compatibility gates

`declarest.compatibleDeclarest` is evaluated at bundle resolution against the running declarest binary version. Resolution fails fast with a validation error when the constraint is not satisfied. The `dev` development build (the default value when binaries are not built through GoReleaser) bypasses the check so in-tree work against unreleased bundles continues to function.

`declarest.compatibleManagedService.{product, versions}` is validated for syntax at decode time but not yet evaluated against a live managed service. Runtime enforcement is deferred until a `managedservice.ProductVersionProvider` capability lands; the field is reserved for that change and MUST stay syntactically valid in published bundles. Both `product` and `versions` are required together.

## Release contract

The bundle release workflow is responsible for stamping the real `version` into the published `bundle.yaml`. The committed copy stays on the placeholder version (for example `0.0.0`). The published archive name MUST match the `distribution.artifactTemplate` derived from the bundle `name`:

- `keycloak-bundle-<version>.tar.gz`
- `rundeck-bundle-<version>.tar.gz`
- `haproxy-bundle-<version>.tar.gz`

The shorthand resolver (`metadata.bundle: <name>:<version>`) downloads the artifact from `https://github.com/crmarques/declarest-bundle-<base>/releases/download/v<version>/<name>-<version>.tar.gz` where `<base>` is the bundle name with the `-bundle` suffix removed.

## Reference forms accepted by `metadata.bundle`

`bundlemetadata.ResolveBundle` accepts four canonical reference forms, resolved in priority order:

1. `oci://<registry>/<repository>:<tag>` or `oci://<registry>/<repository>@sha256:<hex>` — pulls the bundle as an OCI artifact via the `oras-go/v2` client. This is the default reference used by the official declarest metadata bundles published to GHCR, for example `oci://ghcr.io/crmarques/declarest-metadata-bundles/keycloak:0.0.1`. The resolver selects the first layer advertised as `application/vnd.declarest.bundle.v1.tar+gzip` (or equivalent tar+gzip media type) and passes it through the same strict-decode and compatibility-gate pipeline used for other sources. No external `oras` CLI is required.
2. `<name>:<version>` shorthand — resolves through the GitHub-release URL described above (legacy path, kept for backwards compatibility).
3. `http`/`https` URL — downloads the referenced `.tar.gz` directly.
4. Absolute local filesystem path — loads the `.tar.gz` archive from disk (useful for offline development).

Bundle resolution honours `metadata.proxy` for all remote sources (OCI registry, HTTP URL, shorthand) through the shared HTTP client factory.
