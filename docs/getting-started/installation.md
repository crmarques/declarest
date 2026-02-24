# Installation

Keep this page simple: install the CLI, verify it runs, then continue to the quickstart.

## Option 1: Download a release binary (recommended)

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

## Option 2: Build from source

```bash
go build -o bin/declarest ./cmd/declarest
sudo install -m 0755 ./bin/declarest /usr/local/bin/declarest

declarest version
```

## Shell completion (optional, recommended)

The CLI can generate shell completion scripts:

```bash
declarest completion bash
declarest completion zsh
declarest completion fish
declarest completion powershell
```

Common setup examples:

### Bash

```bash
source <(declarest completion bash)
```

Persist by adding the same line to `~/.bashrc`.

### Zsh

```bash
mkdir -p ~/.zfunc
declarest completion zsh > ~/.zfunc/_declarest
print -r -- 'fpath+=(~/.zfunc)' >> ~/.zshrc
print -r -- 'autoload -U compinit && compinit' >> ~/.zshrc
```

### Fish

```bash
mkdir -p ~/.config/fish/completions
declarest completion fish > ~/.config/fish/completions/declarest.fish
```

### PowerShell

```powershell
declarest completion powershell | Out-File -Encoding utf8 $PROFILE.CurrentUserAllHosts
```

## Next step

Go to [Quickstart](quickstart.md) for a minimal happy-flow setup.
