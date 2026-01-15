SHELL := /bin/bash
.DEFAULT_GOAL := help

BIN_DIR := bin
BIN := $(BIN_DIR)/declarest
CMD := ./cli
VERSION ?= $(shell git describe --tags --dirty --always 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X declarest/cli/cmd.Version=$(VERSION) -X declarest/cli/cmd.Commit=$(COMMIT) -X declarest/cli/cmd.Date=$(DATE)

.PHONY: help build test run fmt tidy deps clean

help:
	@grep -E '^[a-zA-Z_-]+:.*?## .+' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build:
	mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN) $(CMD)

test:
	go test -race ./...

run:
	go run $(CMD) $(ARGS)

fmt:
	go fmt ./...

tidy:
	go mod tidy

deps:
	go mod download

clean:
	rm -rf $(BIN_DIR)
