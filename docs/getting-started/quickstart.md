# Quickstart

This walkthrough sets up a context, initializes a repository, and syncs a resource.

## 1) Set up a *context* to define the *target server* connection and the Git-backed repository to use

a) Interactive setup:

```bash
./bin/declarest config init
```

b) Or generate a full config file:

```bash
./bin/declarest config print-template > ./contexts/staging.yaml
```

Edit the file, replace the placeholders, then run:

```bash
./bin/declarest config add staging ./contexts/staging.yaml
./bin/declarest config use staging
```

## 2) Check configuration

```bash
./bin/declarest config check
```

## 3) Init repository

```bash
./bin/declarest repo init
```

## 4) Pull a remote resource into Git

```bash
./bin/declarest resource get --path /projects/example --save
```

This creates a `resource.json` under the repository base directory at:

```
/projects/example/resource.json
```

## 5) Apply changes back to the API

Edit the local `resource.json`, then:

```bash
./bin/declarest resource diff --path /projects/example
./bin/declarest resource apply --path /projects/example
```

For more details, see [Concepts](../concepts/overview.md) and [Configuration](../reference/configuration.md).
