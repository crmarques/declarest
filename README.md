# DeclaREST

<p align="center">
  <img src="docs/assets/logo.png" alt="DeclaREST logo" width="220">
</p>

<p align="center">
  <strong>Declarative resource sync between Git and REST APIs</strong>
</p>

<p align="center">
  <a href="docs/getting-started/quickstart-cli.md">Quickstart</a> &middot;
  <a href="docs/guide/core-concepts.md">Concepts</a> &middot;
  <a href="docs/reference/cli.md">CLI Reference</a> &middot;
  <a href="docs/contributing/contributing.md">Contributing</a>
</p>

---

Managing API configurations usually means scripts, manual UI clicks, and changes nobody can review or reproduce. DeclaREST replaces that with a simple model: **store desired state in Git, and let DeclaREST keep your systems in sync.**

## See it in action

```bash
# 1. Set up a context (interactive — connects a Git repo to a REST API)
declarest context add

# 2. Pull a resource from the API into a local file
declarest resource save /corporations/acme

# 3. Edit the file to match what you want
declarest resource edit /corporations/acme

# 4. See exactly what will change
declarest resource diff /corporations/acme

# 5. Apply the change — DeclaREST handles the HTTP calls
declarest resource apply /corporations/acme
```

That's it. Your configuration now lives in Git with full history, pull requests, and repeatable automation.

## Why DeclaREST?

| Problem | DeclaREST solution |
|---|---|
| API configs are scattered and untracked | Git becomes the single source of truth |
| No review process for infrastructure changes | Every change is a commit you can review in a PR |
| Raw API endpoints vary wildly across products | Stable logical paths like `/corporations/acme` abstract the mess |
| Credentials leak into config files | Secret placeholders (`{{secret .}}`) keep plaintext out of Git |
| Manual sync is error-prone | Deterministic save/diff/apply workflow catches drift |
| Need both local debugging and production sync | CLI for on-demand work, Kubernetes Operator for continuous reconciliation |

## How it works

<p align="center">
  <img src="docs/assets/usage-flow.png" alt="DeclaREST usage flow" width="650">
</p>

A **context** ties everything together: a Git repository, a managed REST API server, optional metadata for API modeling, and a secret provider. From there:

- **Metadata** maps logical paths to real API endpoints, methods, and transforms — so you don't have to think in raw HTTP.
- The **CLI** runs deterministic workflows: save, diff, apply, and more.
- The **Kubernetes Operator** watches your Git repo and reconciles continuously, so production stays in sync without manual intervention.

## Install

Download a release binary or build from source:

```bash
go build -o bin/declarest ./cmd/declarest
./bin/declarest version
```

## What you can do

**Resource workflows** — the core loop:

```
save  diff  apply  get  list  edit  copy  create  update  delete  explain
```

**Metadata workflows** — model any API shape:

```
get  resolve  render  infer  set  unset
```

**Repository workflows** — Git operations without leaving DeclaREST:

```
status  tree  history  commit  refresh  push  reset  clean  check
```

**Secret workflows** — keep credentials safe:

```
detect  store  mask  resolve          # plus safe-save guards on every write
```

**Kubernetes Operator** — continuous GitOps reconciliation with four CRDs:

```
ResourceRepository    ManagedServer    SecretStore    SyncPolicy
```

## Kubernetes Operator

For continuous sync in production, DeclaREST ships a Kubernetes Operator that watches your Git repository and reconciles resources automatically.

```bash
VERSION=vX.Y.Z
kubectl create namespace declarest-system
kubectl apply -f "https://github.com/crmarques/declarest/releases/download/${VERSION}/install.yaml"
```

Release install manifests:

| Manifest | Includes | Extra dependency | Recommended when |
|---|---|---|---|
| `install.yaml` | CRDs, RBAC, manager deployment | None | You want the simplest install on a standard Kubernetes cluster and do not need admission webhooks yet |
| `install-admission-certmanager.yaml` | `install.yaml` plus validating admission webhooks | `cert-manager` installed in the cluster | Recommended for most production Kubernetes clusters because it adds early CR validation without requiring OpenShift |
| `install-admission-openshift.yaml` | `install.yaml` plus validating admission webhooks | OpenShift service serving cert support | Recommended on OpenShift clusters; do not use this on generic Kubernetes |

See the [Operator Quickstart](docs/getting-started/quickstart-operator.md) for a full walkthrough.

## Documentation

| Section | What you'll learn |
|---|---|
| [Quickstart (CLI)](docs/getting-started/quickstart-cli.md) | Install, configure, and run your first save/diff/apply |
| [Quickstart (Operator)](docs/getting-started/quickstart-operator.md) | Deploy the operator and set up continuous sync |
| [Core Concepts](docs/guide/core-concepts.md) | Contexts, resources, metadata, secrets, and Git workflows |
| [Working with Resources](docs/guide/working-with-resources.md) | Day-to-day resource operations in depth |
| [Metadata & API Modeling](docs/guide/metadata-and-api-modeling.md) | Map any REST API to stable logical paths |
| [Managing Secrets](docs/guide/managing-secrets.md) | Placeholder lifecycle and secret providers |
| [Repository & Git Workflows](docs/guide/repository-and-git-workflows.md) | Branching, committing, and pushing from the CLI |
| [Running the Operator](docs/guide/running-the-operator.md) | CRD configuration and production setup |
| [Troubleshooting](docs/topics/troubleshooting.md) | Common issues and diagnosis checklist |

Full reference: [CLI](docs/reference/cli.md) &middot; [Configuration](docs/reference/configuration.md) &middot; [Metadata Schema](docs/reference/metadata-schema.md) &middot; [Operator CRDs](docs/reference/operator-crds.md)

## Contributing

See [Contributing](docs/contributing/contributing.md) for development setup, testing, documentation, and release workflows.
