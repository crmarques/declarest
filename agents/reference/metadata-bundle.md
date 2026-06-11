# Metadata Bundle Manifest Contract

## Purpose
Define the canonical `bundle.yaml` shape consumed by `metadata.bundle` / `metadata.bundleFile`, its strict-decode rules, the compatibility gates evaluated at resolution, and the reference forms resolved by `bundlemetadata.ResolveBundle`. interfaces.md points here for the full shape.

## Scope
Owns the `bundle.yaml` wire shape (`apiVersion: declarest.io/v1alpha1`, `kind: MetadataBundle`), strict decode, compat gates, `distribution.artifactTemplate` naming, and bundle ref-form resolution. Metadata layering/rendering/directives are defined in metadata.md; the error taxonomy in interfaces.md; the proxy model and `${ENV_VAR}` expansion in context-config.md. Archive extraction limits and cache layout live in `internal/providers/metadata/bundle` and are not part of this contract.

## Normative Rules

### Reference resolution
1. `metadata.bundle` reference forms MUST resolve in this priority order:
   1. `oci://<registry>/<repository>:<tag>` or `oci://<registry>/<repository>@<digest>` MUST pull the bundle as an OCI artifact via the `oras-go/v2` client.
   2. `configmap://<namespace>/<name>/<key>` MUST read tarball bytes from the matching `WithInMemoryBundles` entry (controller-only; not a user-facing scheme).
   3. `<name>:<version>` shorthand MUST resolve through the GitHub-release shorthand URL.
   4. An `http`/`https` URL MUST download the referenced tarball directly.
   5. A `file://` URL MUST map to a local filesystem path.
   6. Any other non-empty value MUST be treated as a local filesystem path to a `.tar.gz` archive or to an unpacked directory containing `bundle.yaml` at its root.
2. OCI resolution MUST select the first layer advertised as `application/vnd.declarest.bundle.v1.tar+gzip` (falling back to `application/vnd.oci.image.layer.v1.tar+gzip` and equivalent tar+gzip media types), then MUST pass the layer stream through the same strict-decode and compat-gate pipeline used by every other source kind.
3. OCI registry access MUST honour `metadata.proxy` via the shared HTTP client factory (proxy model: context-config.md) and MUST NOT invoke an external `oras` CLI or any out-of-process tool.
4. OCI registry access MUST prefer credentials from `WithRegistryCredentials([]RegistryCredential)` over ambient docker-config auth. With no caller-supplied credentials the resolver MAY fall back to the default oras-go auth discovery path for developer convenience; operator pods MUST always supply credentials via the option so resolution stays reproducible.
5. Local filesystem resolution MUST detect when the target path is a directory containing a readable `bundle.yaml`; when so, the resolver MUST use the directory in place (no copy/extract into cache) and `BundleResolution.MetadataDir` MUST point at `<directory>/<metadataRoot>`.

### Strict decode and shape
6. `bundle.yaml` MUST decode strictly: any unknown YAML key at any level MUST fail with `ValidationError`.
7. The manifest MUST define `apiVersion: declarest.io/v1alpha1`, `kind: MetadataBundle`, `name`, `version`, `description`, and `declarest.metadataRoot`.
8. `version` MUST be a semver-2 value (optional leading `v`); the canonical normalized form drops the leading `v`.
9. `declarest.metadataRoot` MUST be a repository-relative path that does not traverse parents.
10. `declarest.openapi`, when set, MUST be either a repository-relative path or an `http`/`https` URL; any other scheme MUST fail decoding.
11. Annotations, sources, keywords, license, maintainers, home, OCI annotations, `declarest.shorthand`, `declarest.metadataFileName`, `distribution.repo`, and `distribution.tagTemplate` are NOT part of the consumer contract; authors MUST NOT include them and the strict-decode rule MUST reject them.
12. `distribution.artifactTemplate`, when set, MUST equal `<name>-{version}.tar.gz`; no other `distribution` field is part of the consumer contract.
13. Changes to the persisted wire shape MUST update `schemas/bundle.schema.json` in the same change.

### Compatibility gates
14. `declarest.compatibleDeclarest`, when set, MUST be a `Masterminds/semver` constraint string, validated at decode time.
15. `declarest.compatibleDeclarest` MUST be evaluated at resolution: if the running declarest binary version satisfies the constraint, resolution proceeds; otherwise it MUST fail with `ValidationError` and MUST NOT extract the archive into the cache.
16. Binary version `dev` (unstamped development build) MUST bypass the `compatibleDeclarest` gate; published builds MUST stay gated.
17. `declarest.compatibleManagedService` MUST require both `product` and `versions`; setting one without the other MUST fail decoding.
18. `declarest.compatibleManagedService.product`, when set, MUST be a non-empty identifier matching `^[a-z0-9][a-z0-9-]*$`.
19. `declarest.compatibleManagedService.versions`, when set, MUST be a `Masterminds/semver` constraint string validated at decode time; runtime evaluation against an actual managed-service version is deferred until a `managedservice.ProductVersionProvider` capability lands, and a valid constraint MUST NOT block resolution meanwhile.
20. `deprecated` MAY be set; when `true`, resolution MUST emit a deprecation warning naming the bundle and version.

## Data Contracts

### `bundlemetadata.BundleManifest`
Persisted manifest decoded from `bundle.yaml`.

Required: `APIVersion` (`declarest.io/v1alpha1`), `Kind` (`MetadataBundle`), `Name` (lowercase, hyphenated), `Version` (semver-2, with/without leading `v`), `Description`, `Declarest.MetadataRoot` (repository-relative path to the metadata tree root).

Optional: `Deprecated` (bool); `Declarest.OpenAPI` (repo-relative path or `http`/`https` URL); `Declarest.CompatibleDeclarest` (constraint vs running binary version); `Declarest.CompatibleManagedService.Product` + `.Versions` (paired; runtime check deferred); `Distribution.ArtifactTemplate` (MUST equal `<name>-{version}.tar.gz` when set).

### `bundlemetadata.BundleResolverOption`
Functional options for `ResolveBundle`:
1. `WithProxyConfig(*config.HTTPProxy)` — proxy for remote archive fetches.
2. `WithPromptRuntime(*promptauth.Runtime)` — credential prompt runtime for proxy auth.
3. `WithDeclarestVersion(string)` — binary version for the `compatibleDeclarest` gate; literal `dev` bypasses it.
4. `WithCacheRoot(string)` — overrides the on-disk cache root (default `~/.declarest/metadata-bundles`). Operator deployments SHOULD set a writable-volume path.
5. `WithRegistryCredentials([]RegistryCredential)` — static OCI credentials overriding ambient docker-config auth; empty list preserves default discovery.
6. `WithInMemoryBundles(map[string][]byte)` — registers tarball bytes for internal URL forms (e.g. `configmap://<namespace>/<name>/<key>`), letting the operator pass ConfigMap bytes without leaking Kubernetes types.

### `bundlemetadata.RegistryCredential`
Required: `Registry` (host or `host:port`, e.g. `ghcr.io`), `Username`, `Password` (password or PAT).

### `bundlemetadata.BundleResolution`
Required: `MetadataDir` (absolute path to resolved metadata root), `Manifest` (decoded `BundleManifest`), `Shorthand` (`true` when ref was `<name>:<version>`).
Optional: `OpenAPI` (resolved path/URL or empty), `DeprecatedWarning` (text when `Manifest.Deprecated`).

## Failure Modes
1. `ValidationError` — unknown YAML key, missing required field, malformed semver, malformed constraint, partial `compatibleManagedService`, illegal openapi scheme, parent-traversing `metadataRoot`, unsupported `artifactTemplate`, or running binary version failing `compatibleDeclarest`.
2. `NotFoundError` — `bundle.yaml` missing from the resolved bundle directory.

## Examples
1. OCI form (default deployment): `metadata.bundle: oci://ghcr.io/crmarques/declarest-metadata-bundles/keycloak:0.0.1` MUST fetch the artifact from GHCR, extract the `application/vnd.declarest.bundle.v1.tar+gzip` layer, and strict-decode its manifest like a local tarball.
2. Shorthand form: `metadata.bundle: keycloak-bundle:0.1.0` MUST download GitHub-release artifact `keycloak-bundle-0.1.0.tar.gz` and resolve identically once staged. `version: v1.2.3` and `version: 1.2.3` MUST normalize to `1.2.3` for shorthand matching.
3. Minimal valid bundle:
```yaml
apiVersion: declarest.io/v1alpha1
kind: MetadataBundle
name: keycloak-bundle
version: 0.0.0
description: Metadata bundle for Keycloak Admin REST API.
declarest:
  metadataRoot: metadata
  openapi: openapi.yaml
  compatibleDeclarest: ">=0.1.0"
  compatibleManagedService:
    product: keycloak
    versions: ">=26.0.0 <27.0.0"
distribution:
  artifactTemplate: keycloak-bundle-{version}.tar.gz
```
4. Strict-decode failure: a manifest containing a legacy decorative key (e.g. `keywords`, `home`, `license`, `maintainers`, `annotations`, `declarest.shorthand`, `declarest.metadataFileName`, `distribution.repo`, `distribution.tagTemplate`) MUST fail with a `ValidationError` naming the unknown key. An `artifactTemplate` mismatching `<name>-{version}.tar.gz` MUST fail decoding before resolution.
5. Compat gate vs dev bypass: binary `0.0.5` resolving `compatibleDeclarest: ">=0.1.0"` MUST fail with `ValidationError` without extracting the archive; binary `dev` resolving the same bundle MUST succeed without evaluating the constraint. A deferred `compatibleManagedService.versions: ">=26 <27"` MUST decode and MUST NOT block resolution.
