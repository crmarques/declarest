SHELL := /usr/bin/env bash
.SHELLFLAGS := -o pipefail -ec
MAKEFLAGS += --warn-undefined-variables

GO ?= go
GOFLAGS ?= -mod=readonly
PYTHON ?= python3
GO_VERSION := $(shell awk '/^go /{print $$2; exit}' go.mod)
BIN_DIR := bin
BINARY := $(BIN_DIR)/declarest
OPERATOR_BINARY := $(BIN_DIR)/declarest-operator-manager
OPERATOR_IMAGE ?= declarest-operator
OPERATOR_IMAGE_TAG ?= latest
OPERATOR_IMAGE_REF := $(OPERATOR_IMAGE):$(OPERATOR_IMAGE_TAG)
TEST_FLAGS ?= -race
E2E_FLAGS ?=
DOCS_SITE_DIRS := site docs/site .docs
DOCS_VENV_DIR := .venv
DOCS_REQUIREMENTS := docs/requirements.txt
DOCS_VENV_PYTHON := $(DOCS_VENV_DIR)/bin/python
DOCS_VENV_MKDOCS := $(DOCS_VENV_DIR)/bin/mkdocs
DOCS_VENV_STAMP := $(DOCS_VENV_DIR)/.requirements.stamp
E2E_RUNS_DIR := test/e2e/.runs
E2E_BUILD_DIR := .e2e-build

.DEFAULT_GOAL := help

.PHONY: help fmt vet lint test docs docs-deps e2e e2e-contract e2e-validate-components check build run install clean tidy operator-build operator-run operator-test operator-image operator-image-push

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

$(DOCS_VENV_PYTHON):
	$(PYTHON) -m venv $(DOCS_VENV_DIR)

$(DOCS_VENV_STAMP): $(DOCS_VENV_PYTHON) $(DOCS_REQUIREMENTS)
	$(DOCS_VENV_PYTHON) -m pip install --requirement $(DOCS_REQUIREMENTS)
	@touch $(DOCS_VENV_STAMP)

docs-deps: $(DOCS_VENV_STAMP) ## Create or refresh the MkDocs virtualenv dependencies

docs: docs-deps ## Build the MkDocs documentation site locally
	@$(DOCS_VENV_MKDOCS) build --strict --clean --site-dir .docs

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
	podman build --build-arg GO_VERSION=$(GO_VERSION) -f Dockerfile.operator -t $(OPERATOR_IMAGE_REF) .

operator-image-push: ## Push the operator manager container image
	podman push $(OPERATOR_IMAGE_REF)

install: ## Install the CLI into $(GOBIN) or GOPATH/bin
	$(GO) install ./cmd/declarest

clean: ## Remove build artifacts and transient docs/e2e outputs
	test/e2e/run-e2e.sh --clean-all
	@if command -v deactivate >/dev/null 2>&1; then \
		deactivate; \
	fi
	rm -rf $(BIN_DIR) $(DOCS_SITE_DIRS) $(E2E_RUNS_DIR) $(E2E_BUILD_DIR) $(DOCS_VENV_DIR)
	@if [ -d test/e2e/components ]; then \
		find test/e2e/components -type d -name '__pycache__' -prune -exec rm -rf {} +; \
	fi

tidy: ## Reconcile go.mod and go.sum with the current imports
	$(GO) mod tidy
