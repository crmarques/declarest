# Installation

DeclaREST is a Go-based CLI. 

The easiest way to install it is to download a release binary (**Option 1**).

## **Option 1:** Download from GitHub releases (recommended)

1) Download the binary for your OS and architecture from:

`https://github.com/crmarques/declarest/releases`

2) Install it into `/usr/local/bin`:

```bash
VERSION=vX.Y.Z
ARCHIVE=declarest_${VERSION//v}_linux_amd64.tar.gz

curl -L -o /tmp/${ARCHIVE} \
  https://github.com/crmarques/declarest/releases/download/${VERSION}/${ARCHIVE}
tar -xzf /tmp/${ARCHIVE} -C /tmp
sudo install -m 0755 /tmp/declarest /usr/local/bin/declarest

declarest --help
```

Use the matching archive name for your platform (for example, `darwin_amd64` or `darwin_arm64`).

## **Option 2:** Build from source

From the repository root:

```bash
make build
sudo install -m 0755 ./bin/declarest /usr/local/bin/declarest
declarest --help
```

Without `make`:

```bash
go build -o bin/declarest ./cli
sudo install -m 0755 ./bin/declarest /usr/local/bin/declarest
declarest --help
```

## Configuration store location

Contexts are stored in `~/.declarest/config` by default.
Use `declarest config` commands to manage them.

## Shell completion

`declarest completion <shell>` prints a completion script for `bash`, `zsh`, `fish`, or PowerShell. Pipe or source the script in your shell so you can press Tab to complete commands, paths, and OpenAPI templates.

### Bash

- One-time:
  ```bash
  source <(declarest completion bash)
  ```
- Persist across sessions by adding the same command to `~/.bashrc` or `~/.bash_profile`:
  ```bash
  echo 'source <(declarest completion bash)' >> ~/.bashrc
  ```
  and reload the file with `source ~/.bashrc` (or restart the shell).

### Zsh

- Write the completion function to your `fpath` directory:
  ```bash
  mkdir -p ~/.zfunc
  declarest completion zsh > ~/.zfunc/_declarest
  ```
- Update `~/.zshrc` if needed:
  ```bash
  echo 'fpath+=(~/.zfunc)' >> ~/.zshrc
  echo 'autoload -U compinit && compinit' >> ~/.zshrc
  ```
  Then restart zsh or run `source ~/.zshrc`.

### Fish

- Install the completion script into the fish completions directory:
  ```bash
  mkdir -p ~/.config/fish/completions
  declarest completion fish > ~/.config/fish/completions/declarest.fish
  ```
  Fish automatically loads any file under `~/.config/fish/completions`, so a new shell session will honor the completions.

### PowerShell

- Save the completion script under your AllUsers profile and reload it at startup:
  ```powershell
  $profilePath = Join-Path $PROFILE.CurrentUserAllHosts 'declarest.ps1'
  declarest completion powershell | Out-File -Encoding utf8 $profilePath
  . $profilePath
  ```
  Restart PowerShell (or dot-source the file) to activate the completions for every session.
