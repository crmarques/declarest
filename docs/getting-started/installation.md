# Installation

DeclaREST is a Go-based CLI. 

The easiest way to install it is to download a release binary (**Option 1**).

## **Option 1:** Download from GitHub releases (recommended)

1) Download the binary for your OS and architecture from:

`https://github.com/crmarques/declarest/releases`

2) Install it into `/usr/local/bin`:

```bash
VERSION=vX.Y.Z
ARCHIVE=declarest_${VERSION}_linux_amd64.tar.gz

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
