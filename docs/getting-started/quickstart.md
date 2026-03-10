# Quickstart (CLI)

This page installs the CLI and walks through one happy-flow setup: one context, one resource, one edit, one apply.

## 1. Install the CLI

If you already have `declarest` available in your shell, skip to step 2.

### Option A: Download a release binary

Releases are published at:

- `https://github.com/crmarques/declarest/releases`

Example for Linux amd64:

```bash
VERSION=vX.Y.Z
ARCHIVE="declarest_${VERSION#v}_linux_amd64.tar.gz"

curl -L -o "/tmp/${ARCHIVE}" \
  "https://github.com/crmarques/declarest/releases/download/${VERSION}/${ARCHIVE}"

tar -xzf "/tmp/${ARCHIVE}" -C /tmp
sudo install -m 0755 /tmp/declarest /usr/local/bin/declarest

declarest version
```

Adjust the archive name for your platform (`darwin_arm64`, `darwin_amd64`, `windows_amd64.zip`, etc.).

### Option B: Build from source

```bash
go build -o bin/declarest ./cmd/declarest
sudo install -m 0755 ./bin/declarest /usr/local/bin/declarest

declarest version
```

### Shell completion (optional)

The CLI can generate shell completion scripts:

```bash
declarest completion bash
declarest completion zsh
declarest completion fish
declarest completion powershell
```

Common setup examples:

#### Bash

```bash
source <(declarest completion bash)
```

Persist by adding the same line to `~/.bashrc`.

For `bash-completion` based setups, you can install it permanently as:

```bash
mkdir -p ~/.local/share/bash-completion/completions
declarest completion bash > ~/.local/share/bash-completion/completions/declarest
```

#### Zsh

```bash
mkdir -p ~/.zfunc
declarest completion zsh > ~/.zfunc/_declarest
print -r -- 'fpath+=(~/.zfunc)' >> ~/.zshrc
print -r -- 'autoload -U compinit && compinit' >> ~/.zshrc
```

#### Fish

```bash
mkdir -p ~/.config/fish/completions
declarest completion fish > ~/.config/fish/completions/declarest.fish
```

#### PowerShell

```powershell
declarest completion powershell | Out-File -Encoding utf8 $PROFILE.CurrentUserAllHosts
```

## 2. Create a context

A context tells DeclaREST where your repository lives and how to reach the managed server.

Interactive (recommended for first use):

```bash
declarest context add
```

If you prefer file-based setup:

```bash
declarest context print-template > /tmp/contexts.yaml
# edit /tmp/contexts.yaml
declarest context add --payload /tmp/contexts.yaml --set-current
```

Check the active configuration:

```bash
declarest context current
declarest context check
```

## 3. Save one resource from the API into the repository

Pull the current remote state so you have a baseline to work from:

```bash
declarest resource save /corporations/acme
```

This writes the payload to the repository base dir configured in your context, for example:

- `<repo-base-dir>/corporations/acme/resource.json`
- or another `resource.<ext>` when the managed server responds with a different media type

## 4. Inspect and edit locally

Review and modify the saved resource to define your desired state:

```bash
declarest resource get --source repository /corporations/acme
# edit the file in your editor
```

## 5. Diff and apply

Compare your local changes against the live server, then push them:

```bash
declarest resource diff /corporations/acme
declarest resource apply /corporations/acme
```

## 6. Verify

Confirm the remote state matches what you applied:

```bash
declarest resource get /corporations/acme
```

## What to learn next

- [Contexts](../concepts/context.md) — understand how contexts connect everything
- [Paths and Selectors](../concepts/paths-and-selectors.md) — learn the path model
- [Metadata overview](../concepts/metadata.md) — adapt DeclaREST to non-REST APIs
- [Configuration reference](../reference/configuration.md) — all context options
