SHELL := /usr/bin/env bash
.SHELLFLAGS := -o pipefail -ec
MAKEFLAGS += --warn-undefined-variables

GO ?= go
GOFLAGS ?= -mod=readonly
BIN_DIR := bin
BINARY := $(BIN_DIR)/declarest
TEST_FLAGS ?= -race

.DEFAULT_GOAL := help

.PHONY: help fmt vet lint test check build run install clean tidy

help: ## List available make targets with descriptions
	@printf "Available targets:\n"
	@grep -E '^[a-zA-Z0-9_/.@-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS=":.*?## ";} {printf "  %-20s %s\n", $$1, $$2}'

fmt: ## Run gofmt via go fmt on all packages
	$(GO) fmt ./...

vet: ## Run go vet to surface suspicious constructs
	$(GO) vet ./...

lint: ## Run golangci-lint if available, otherwise fall back to go vet
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		$(MAKE) vet; \
	fi

test: ## Run the test suite with race detection
	$(GO) test $(TEST_FLAGS) ./...

check: fmt lint test ## Run formatting, linting, and tests

build: ## Compile the declarest binary into $(BIN_DIR)/
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BINARY) ./cmd/declarest

run: ## Build and run the CLI via go run
	$(GO) run ./cmd/declarest

install: ## Install the CLI into $(GOBIN) or GOPATH/bin
	$(GO) install ./cmd/declarest

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)

tidy: ## Reconcile go.mod and go.sum with the current imports
	$(GO) mod tidy
