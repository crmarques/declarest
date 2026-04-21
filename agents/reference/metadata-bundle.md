# Metadata Bundle Manifest Contract

## Purpose
Define the canonical `bundle.yaml` shape consumed by `metadata.bundle` / `metadata.bundleFile`, the strict-decode rules that lock that shape, and the compatibility gates evaluated when a bundle is loaded.

## In Scope
1. The `bundle.yaml` wire shape under `apiVersion: declarest.io/v1alpha1`, `kind: MetadataBundle`.
2. Strict YAML decoding for `bundle.yaml`.
3. `declarest.compatibleDeclarest` runtime evaluation.
4. `declarest.compatibleManagedService` declaration shape and syntactic validation.
5. The release-time naming contract enforced by `distribution.artifactTemplate`.
6. The supported `metadata.bundle` reference forms resolved by `bundlemetadata.ResolveBundle` (OCI registry artifact, shorthand, `http`/`https` URL, `file://` URL, local tarball or directory path, and the operator-internal `configmap://` form).

## Out of Scope
1. Metadata layering, template rendering, and operation directives (see `agents/reference/metadata.md`).
2. Bundle archive extraction safety limits and cache layout (kept inside `internal/providers/metadata/bundle`).
3. Per-product runtime version probing (deferred; managed-service version checks remain syntax-only until a `managedservice.ProductVersionProvider` capability lands).
4. Bundle repository GitHub release workflow internals beyond the manifest fields they read.

## Normative Rules
0. `metadata.bundle` reference forms MUST be resolved in this priority order: an `oci://<registry>/<repository>:<tag>` or `oci://<registry>/<repository>@<digest>` reference MUST pull the bundle as an OCI artifact via the `oras-go/v2` client; a `configmap://<namespace>/<name>/<key>` reference MUST read tarball bytes from the corresponding entry supplied via `WithInMemoryBundles` (controller-only, not a user-facing scheme); a `<name>:<version>` shorthand MUST continue to resolve through the GitHub-release shorthand URL; an `http`/`https` URL MUST download the referenced tarball directly; a `file://` URL MUST map to a local filesystem path; any other non-empty value MUST be treated as a local filesystem path to a `.tar.gz` archive or to an unpacked directory containing `bundle.yaml` at its root.
0. OCI-backed resolution MUST select the first layer advertised as `application/vnd.declarest.bundle.v1.tar+gzip` (falling back to `application/vnd.oci.image.layer.v1.tar+gzip` and equivalent tar+gzip media types) and MUST pass the layer stream through the same strict-decode and compatibility-gate pipeline used by other source kinds.
0. OCI registry access MUST honour `metadata.proxy` settings through the shared HTTP client factory and MUST NOT rely on an external `oras` CLI or any other out-of-process tool.
0. OCI registry access MUST prefer credentials supplied through `WithRegistryCredentials([]RegistryCredential)` over any ambient docker-config auth. When no caller-supplied credentials exist the resolver MAY fall back to the default oras-go auth discovery path for developer convenience; operator pods MUST always supply credentials via the option so the resolver remains reproducible.
0. Local filesystem resolution MUST detect when the target path is a directory containing a readable `bundle.yaml`. When so detected the resolver MUST use the directory in place without copying or extracting into the cache, and `BundleResolution.MetadataDir` MUST point at `<directory>/<metadataRoot>`.
1. `bundle.yaml` MUST decode strictly: unknown YAML keys at any level MUST fail decoding with a `ValidationError`.
2. The persisted manifest MUST define `apiVersion: declarest.io/v1alpha1`, `kind: MetadataBundle`, `name`, `version`, `description`, and `declarest.metadataRoot`.
3. `version` MUST be a semver-2 value (with optional leading `v`); the canonical normalized form drops the leading `v`.
4. `declarest.metadataRoot` MUST be a repository-relative path that does not traverse parents.
5. `declarest.openapi`, when set, MUST be either a repository-relative path or an `http`/`https` URL; other URL schemes MUST fail decoding.
6. `declarest.compatibleDeclarest`, when set, MUST be a `Masterminds/semver` constraint string; constraint syntax MUST be validated at decode time.
7. `declarest.compatibleDeclarest` MUST be evaluated at bundle resolution: when the running declarest binary version satisfies the constraint, resolution MUST proceed; otherwise resolution MUST fail with `ValidationError`.
8. The declarest binary version `dev` (the unstamped development build) MUST bypass the `compatibleDeclarest` gate to keep local development against in-tree bundles working.
9. `declarest.compatibleManagedService.product`, when set, MUST be a non-empty lowercase identifier matching `^[a-z0-9][a-z0-9-]*$`.
10. `declarest.compatibleManagedService.versions`, when set, MUST be a `Masterminds/semver` constraint string and MUST be validated at decode time; runtime evaluation against an actual managed-service version is deferred until a `managedservice.ProductVersionProvider` capability is available.
11. `declarest.compatibleManagedService` MUST require both `product` and `versions` together; setting one without the other MUST fail decoding.
12. `distribution.artifactTemplate`, when set, MUST equal `<name>-{version}.tar.gz`; no other distribution fields are part of the consumer contract.
13. The boolean `deprecated` MAY be set; when `true`, bundle resolution MUST emit a deprecation warning that names the bundle and version.
14. Annotations, sources, keywords, license, maintainers, home, OCI annotations, `declarest.shorthand`, `declarest.metadataFileName`, `distribution.repo`, and `distribution.tagTemplate` are NOT part of the consumer contract; bundle authors MUST NOT include them, and decoding MUST reject them via the strict-decode rule.
15. Changes to the persisted bundle wire shape MUST update `schemas/bundle.schema.json` in the same change.

## Data Contracts

### Type: `bundlemetadata.BundleManifest`
Persisted bundle manifest decoded from `bundle.yaml`.

Required fields:
1. `APIVersion` — fixed string `declarest.io/v1alpha1`.
2. `Kind` — fixed string `MetadataBundle`.
3. `Name` — bundle identifier (lowercase, hyphenated).
4. `Version` — semver-2 value, persisted with or without leading `v`.
5. `Description` — free-form summary.
6. `Declarest.MetadataRoot` — repository-relative path to the metadata tree root.

Optional fields:
1. `Deprecated` — boolean.
2. `Declarest.OpenAPI` — repository-relative path or `http`/`https` URL.
3. `Declarest.CompatibleDeclarest` — semver constraint string evaluated against the running declarest binary version.
4. `Declarest.CompatibleManagedService.Product` — lowercase product identifier (paired with `Versions`).
5. `Declarest.CompatibleManagedService.Versions` — semver constraint string (paired with `Product`); runtime evaluation deferred.
6. `Distribution.ArtifactTemplate` — release artifact name template; MUST equal `<name>-{version}.tar.gz` when set.

### Type: `bundlemetadata.BundleResolverOption`
Functional options accepted by `bundlemetadata.ResolveBundle`.

Members:
1. `WithProxyConfig(*config.HTTPProxy)` — proxy used for remote archive fetches.
2. `WithPromptRuntime(*promptauth.Runtime)` — credential prompt runtime for proxy auth.
3. `WithDeclarestVersion(string)` — declarest binary version used for the `compatibleDeclarest` gate; the literal string `dev` bypasses the gate.
4. `WithCacheRoot(string)` — overrides the on-disk cache root (default `~/.declarest/metadata-bundles`). Operator deployments SHOULD set this to a path on a writable volume.
5. `WithRegistryCredentials([]RegistryCredential)` — installs static OCI registry credentials that override any ambient docker-config auth; an empty list preserves the default discovery path.
6. `WithInMemoryBundles(map[string][]byte)` — registers tarball bytes for internal URL forms (for example `configmap://<namespace>/<name>/<key>`). The operator controller uses this to hand ConfigMap-sourced bundle bytes to the provider without leaking Kubernetes types.

### Type: `bundlemetadata.RegistryCredential`
Static username/password pair for an OCI registry host.

Required fields:
1. `Registry` — host or `host:port` of the OCI registry (for example `ghcr.io`).
2. `Username` — registry username.
3. `Password` — registry password or personal access token.

### Type: `bundlemetadata.BundleResolution`
Resolved bundle returned by `bundlemetadata.ResolveBundle`.

Required fields:
1. `MetadataDir` — absolute filesystem path to the resolved metadata root.
2. `Manifest` — fully decoded `BundleManifest`.
3. `Shorthand` — `true` when the source ref was `<name>:<version>`.

Optional fields:
1. `OpenAPI` — resolved OpenAPI source (path or URL) or empty when none was discovered.
2. `DeprecatedWarning` — deprecation warning text when `Manifest.Deprecated` is `true`.

## Failure Modes
1. `ValidationError` — unknown YAML key, missing required field, malformed semver, malformed constraint string, partial `compatibleManagedService`, illegal openapi scheme, illegal `metadataRoot`, or unsupported `distribution.artifactTemplate`.
2. `ValidationError` — running declarest version does not satisfy `compatibleDeclarest`.
3. `NotFoundError` — `bundle.yaml` is missing from the resolved bundle directory.

## Edge Cases
1. `version: v1.2.3` and `version: 1.2.3` MUST resolve to the same canonical normalized version `1.2.3` for shorthand matching.
2. Declarest binary version `dev` MUST bypass `compatibleDeclarest` even when the bundle pins a strict range; published builds MUST still be gated.
3. `declarest.compatibleManagedService.versions: ">=26 <27"` MUST decode successfully today and MUST NOT block bundle resolution while the runtime probe capability is absent; the constraint is reserved for future enforcement and MUST stay syntactically valid.
4. A `bundle.yaml` carrying any legacy decorative key (for example `home`, `keywords`, `license`, `maintainers`, `annotations`, `declarest.shorthand`, `declarest.metadataFileName`, `distribution.repo`, `distribution.tagTemplate`) MUST fail strict decoding so removal is enforced.
5. A bundle archive whose `distribution.artifactTemplate` mismatches `<name>-{version}.tar.gz` MUST fail decoding before any resolution step.

## Examples
0. Canonical OCI reference form resolved by the default deployment: `metadata.bundle: oci://ghcr.io/crmarques/declarest-metadata-bundles/keycloak:0.0.1` MUST fetch the OCI artifact from GHCR, extract the `application/vnd.declarest.bundle.v1.tar+gzip` layer, and verify its manifest against the same strict-decode rules used for local tarballs.
0. Shorthand form continues to work: `metadata.bundle: keycloak-bundle:0.1.0` MUST download the GitHub-release artifact `keycloak-bundle-0.1.0.tar.gz` and resolve identically to the OCI path once the tarball has been staged.
1. Minimal valid bundle:
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
2. Strict-decode failure: a `bundle.yaml` containing `keywords: [declarest, keycloak]` MUST fail with a `ValidationError` that names the unknown key.
3. Compatibility gate: declarest binary `0.0.5` resolving a bundle with `declarest.compatibleDeclarest: ">=0.1.0"` MUST fail with `ValidationError` and MUST NOT extract the archive into the cache.
4. Development bypass: declarest binary `dev` resolving the same bundle MUST succeed without evaluating the constraint.
