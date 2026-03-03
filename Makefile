SHELL := /usr/bin/env bash
.SHELLFLAGS := -o pipefail -ec
MAKEFLAGS += --warn-undefined-variables

GO ?= go
GOFLAGS ?= -mod=readonly
BIN_DIR := bin
BINARY := $(BIN_DIR)/declarest
OPERATOR_BINARY := $(BIN_DIR)/declarest-operator-manager
TEST_FLAGS ?= -race
E2E_FLAGS ?=

.DEFAULT_GOAL := help

.PHONY: help fmt vet lint test e2e e2e-contract e2e-validate-components check build run install clean tidy operator-build operator-run operator-test operator-image

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

e2e: ## Run end-to-end tests (supports E2E_FLAGS='--profile full ...')
	bash ./run-e2e.sh $(E2E_FLAGS)

e2e-contract: ## Run fast Bash e2e harness contract tests
	bash ./test/e2e/tests/run.sh

e2e-validate-components: ## Validate all e2e component contracts and fixtures
	bash ./test/e2e/run-e2e.sh --validate-components

check: fmt lint test ## Run formatting, linting, and tests

build: ## Compile the declarest binary into $(BIN_DIR)/
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(BINARY) ./cmd/declarest

operator-build: ## Compile the declarest operator manager binary into $(BIN_DIR)/
	@mkdir -p $(BIN_DIR)
	$(GO) build $(GOFLAGS) -o $(OPERATOR_BINARY) ./cmd/declarest-operator-manager

run: ## Build and run the CLI via go run
	$(GO) run ./cmd/declarest

operator-run: ## Run the operator manager via go run
	$(GO) run ./cmd/declarest-operator-manager

operator-test: ## Run operator-focused unit tests
	$(GO) test $(TEST_FLAGS) ./api/v1alpha1 ./internal/operator/...

operator-image: ## Build the operator manager container image
	podman build -f Dockerfile.operator -t declarest-operator:latest .

install: ## Install the CLI into $(GOBIN) or GOPATH/bin
	$(GO) install ./cmd/declarest

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)

tidy: ## Reconcile go.mod and go.sum with the current imports
	$(GO) mod tidy
