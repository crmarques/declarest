SHELL := /usr/bin/env bash
.SHELLFLAGS := -o pipefail -ec
MAKEFLAGS += --warn-undefined-variables

GO ?= go
GOFLAGS ?= -mod=readonly
PYTHON ?= python3
GO_VERSION := $(shell awk '/^go /{print $$2; exit}' go.mod)
BIN_DIR := bin
BIN_DIR_ABS := $(abspath $(BIN_DIR))
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

.PHONY: help fmt vet lint test docs docs-deps e2e e2e-contract e2e-validate-components check build run install clean tidy operator-build operator-run operator-test operator-image operator-image-push manifests generate bundle-install-core bundle-install-admission-certmanager bundle-install-admission-openshift bundle-install-olm release-assets stage-release-assets verify-generated bundle release-bundle verify-release-version bundle-build bundle-push bundle-validate bundle-run catalog catalog-build catalog-push catalog-validate olm-install olm-uninstall operator-sdk opm verify-bundle

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
	@PYTHONDONTWRITEBYTECODE=1 $(DOCS_VENV_MKDOCS) build --strict --clean --site-dir .docs

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
	find docs test/e2e/components -type d -name '__pycache__' -prune -exec rm -rf {} + 2>/dev/null || true

tidy: ## Reconcile go.mod and go.sum with the current imports
	$(GO) mod tidy

# --- Operator manifest generation ---

CONTROLLER_GEN_VERSION ?= v0.20.1
CONTROLLER_GEN_BIN := $(BIN_DIR_ABS)/controller-gen
CONTROLLER_GEN ?= $(CONTROLLER_GEN_BIN)
RELEASE_TAG ?= latest

.PHONY: controller-gen

controller-gen:
	@if [ -x "$(CONTROLLER_GEN)" ]; then \
		exit 0; \
	fi; \
	if [ "$(CONTROLLER_GEN)" != "$(CONTROLLER_GEN_BIN)" ]; then \
		echo "ERROR: CONTROLLER_GEN points to a missing executable: $(CONTROLLER_GEN)"; \
		exit 1; \
	fi; \
	mkdir -p "$(BIN_DIR_ABS)"; \
	GOBIN="$(BIN_DIR_ABS)" $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_GEN_VERSION)

manifests: controller-gen ## Regenerate CRD manifests from Go types
	$(CONTROLLER_GEN) crd paths="./api/v1alpha1/..." output:crd:artifacts:config=config/crd/bases

generate: controller-gen ## Regenerate deepcopy methods
	$(CONTROLLER_GEN) object paths="./api/v1alpha1/..."

bundle-install-core: ## Generate dist/install.yaml from core kustomize overlay
	@mkdir -p dist
	sed -i 's|newTag: .*|newTag: $(RELEASE_TAG)|' config/release/core/kustomization.yaml
	kubectl kustomize config/release/core > dist/install.yaml
	@git checkout config/release/core/kustomization.yaml 2>/dev/null || sed -i 's|newTag: .*|newTag: RELEASE_TAG|' config/release/core/kustomization.yaml

bundle-install-admission-certmanager: ## Generate dist/install-admission-certmanager.yaml
	@mkdir -p dist
	sed -i 's|newTag: .*|newTag: $(RELEASE_TAG)|' config/release/admission-certmanager/kustomization.yaml
	kubectl kustomize config/release/admission-certmanager > dist/install-admission-certmanager.yaml
	@git checkout config/release/admission-certmanager/kustomization.yaml 2>/dev/null || sed -i 's|newTag: .*|newTag: RELEASE_TAG|' config/release/admission-certmanager/kustomization.yaml

bundle-install-admission-openshift: ## Generate dist/install-admission-openshift.yaml
	@mkdir -p dist
	sed -i 's|newTag: .*|newTag: $(RELEASE_TAG)|' config/release/admission-openshift/kustomization.yaml
	kubectl kustomize config/release/admission-openshift > dist/install-admission-openshift.yaml
	@git checkout config/release/admission-openshift/kustomization.yaml 2>/dev/null || sed -i 's|newTag: .*|newTag: RELEASE_TAG|' config/release/admission-openshift/kustomization.yaml

bundle-install-olm: ## Generate dist/install-olm.yaml from config/olm overlay (OperatorGroup, CatalogSource, Subscription)
	@mkdir -p dist
	$(eval CATALOG_TAG := $(patsubst v%,%,$(RELEASE_TAG)))
	sed -i 's|image: ghcr.io/crmarques/declarest-operator-catalog:.*|image: ghcr.io/crmarques/declarest-operator-catalog:$(CATALOG_TAG)|' config/olm/catalogsource.yaml
	kubectl kustomize config/olm > dist/install-olm.yaml
	@git checkout config/olm/catalogsource.yaml 2>/dev/null || sed -i 's|image: ghcr.io/crmarques/declarest-operator-catalog:.*|image: ghcr.io/crmarques/declarest-operator-catalog:latest|' config/olm/catalogsource.yaml

release-assets: bundle-install-core bundle-install-admission-certmanager bundle-install-admission-openshift bundle-install-olm ## Generate all release install bundles under dist/

stage-release-assets: ## Stage GoReleaser install, bundle, and catalog assets under .release-assets/
	$(MAKE) release-assets RELEASE_TAG=$(RELEASE_TAG)
	$(MAKE) release-bundle VERSION=$(VERSION)
	@mkdir -p .release-assets
	cp dist/install.yaml .release-assets/install.yaml
	cp dist/install-admission-certmanager.yaml .release-assets/install-admission-certmanager.yaml
	cp dist/install-admission-openshift.yaml .release-assets/install-admission-openshift.yaml
	cp dist/install-olm.yaml .release-assets/install-olm.yaml
	tar -czf .release-assets/declarest-operator-bundle.tar.gz bundle bundle.Dockerfile
	cp bundle/manifests/declarest-operator.clusterserviceversion.yaml .release-assets/declarest-operator-bundle.yaml
	cp catalog/declarest-operator/catalog.yaml .release-assets/declarest-operator-catalog.yaml

verify-generated: manifests generate ## Verify generated files are up-to-date
	@if ! git diff --quiet -- config/crd/bases/ api/v1alpha1/zz_generated.deepcopy.go; then \
		echo "ERROR: Generated files are out of date. Run 'make manifests generate' and commit the result."; \
		git diff --stat -- config/crd/bases/ api/v1alpha1/zz_generated.deepcopy.go; \
		exit 1; \
	fi

# --- OLM bundle and catalog ---

VERSION ?= 0.0.1
CHANNELS ?= alpha
DEFAULT_CHANNEL ?= alpha
IMAGE_TAG_BASE ?= ghcr.io/crmarques/declarest-operator
BUNDLE_CREATED_AT ?= 1970-01-01T00:00:00Z
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:$(VERSION)
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:$(VERSION)
OPERATOR_IMG ?= $(IMAGE_TAG_BASE):$(VERSION)
BUNDLE_GEN_FLAGS ?= -q --manifests --version $(VERSION) --channels $(CHANNELS) --default-channel $(DEFAULT_CHANNEL)
BUNDLE_IMAGE_BUILDER ?= podman
OPM_VERSION ?= v1.65.0
OPERATOR_SDK_VERSION ?= v1.42.2
OPM := $(BIN_DIR_ABS)/opm
OPERATOR_SDK := $(BIN_DIR_ABS)/operator-sdk
OPM_URL := https://github.com/operator-framework/operator-registry/releases/download/$(OPM_VERSION)/linux-amd64-opm
OPERATOR_SDK_URL := https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_linux_amd64

opm: ## Install opm locally into bin/ when missing
	@if [ -x "$(OPM)" ] && "$(OPM)" version 2>/dev/null | grep -q "$(OPM_VERSION)"; then \
		exit 0; \
	fi; \
	mkdir -p "$(BIN_DIR_ABS)"; \
	echo "Downloading opm $(OPM_VERSION) to $(OPM)"; \
	curl -fsSL "$(OPM_URL)" -o "$(OPM)"; \
	chmod +x "$(OPM)"

operator-sdk: ## Install operator-sdk locally into bin/ when missing
	@if [ -x "$(OPERATOR_SDK)" ] && "$(OPERATOR_SDK)" version 2>/dev/null | grep -q "$(OPERATOR_SDK_VERSION)"; then \
		exit 0; \
	fi; \
	mkdir -p "$(BIN_DIR_ABS)"; \
	echo "Downloading operator-sdk $(OPERATOR_SDK_VERSION) to $(OPERATOR_SDK)"; \
	curl -fsSL "$(OPERATOR_SDK_URL)" -o "$(OPERATOR_SDK)"; \
	chmod +x "$(OPERATOR_SDK)"

bundle: manifests generate operator-sdk ## Regenerate OLM bundle manifests under bundle/ for the current VERSION
	sed -i 's|newTag: .*|newTag: $(VERSION)|' config/release/core/kustomization.yaml config/manifests/kustomization.yaml
	kubectl kustomize config/manifests | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	sed -i 's|containerImage: .*|containerImage: $(OPERATOR_IMG)|' bundle/manifests/declarest-operator.clusterserviceversion.yaml
	sed -i 's|createdAt: .*|createdAt: "$(BUNDLE_CREATED_AT)"|' bundle/manifests/declarest-operator.clusterserviceversion.yaml
	@git checkout config/release/core/kustomization.yaml 2>/dev/null || sed -i 's|newTag: .*|newTag: RELEASE_TAG|' config/release/core/kustomization.yaml
	$(MAKE) bundle-validate

bundle-validate: operator-sdk ## Run operator-sdk bundle validate against bundle/ (registry+v1 + operatorframework checks)
	$(OPERATOR_SDK) bundle validate ./bundle --select-optional suite=operatorframework

release-bundle: ## Regenerate and validate bundle/catalog artifacts for VERSION
	$(MAKE) bundle VERSION=$(VERSION)
	$(MAKE) catalog VERSION=$(VERSION)
	kubectl kustomize config/olm >/dev/null
	$(MAKE) catalog-validate
	$(MAKE) verify-release-version VERSION=$(VERSION)

verify-release-version: ## Verify generated OLM artifacts point at VERSION and matching images
	@grep -Fq "name: declarest-operator.v$(VERSION)" bundle/manifests/declarest-operator.clusterserviceversion.yaml || { echo "ERROR: bundle CSV name does not match VERSION=$(VERSION)"; exit 1; }
	@grep -Fq "version: $(VERSION)" bundle/manifests/declarest-operator.clusterserviceversion.yaml || { echo "ERROR: bundle CSV version does not match VERSION=$(VERSION)"; exit 1; }
	@grep -Fq "image: $(OPERATOR_IMG)" bundle/manifests/declarest-operator.clusterserviceversion.yaml || { echo "ERROR: bundle CSV manager image does not match $(OPERATOR_IMG)"; exit 1; }
	@grep -Fq "containerImage: $(OPERATOR_IMG)" bundle/manifests/declarest-operator.clusterserviceversion.yaml || { echo "ERROR: bundle CSV containerImage annotation does not match $(OPERATOR_IMG)"; exit 1; }
	@grep -Fq "name: declarest-operator.v$(VERSION)" catalog/declarest-operator/catalog.yaml || { echo "ERROR: catalog bundle name does not match VERSION=$(VERSION)"; exit 1; }
	@grep -Fq "version: $(VERSION)" catalog/declarest-operator/catalog.yaml || { echo "ERROR: catalog package version does not match VERSION=$(VERSION)"; exit 1; }
	@grep -Fq "image: $(BUNDLE_IMG)" catalog/declarest-operator/catalog.yaml || { echo "ERROR: catalog bundle image does not match $(BUNDLE_IMG)"; exit 1; }

bundle-build: ## Build the OLM bundle image ($(BUNDLE_IMG))
	$(BUNDLE_IMAGE_BUILDER) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

bundle-push: ## Push the OLM bundle image ($(BUNDLE_IMG))
	$(BUNDLE_IMAGE_BUILDER) push $(BUNDLE_IMG)

bundle-run: operator-sdk ## Install the bundle into the current cluster via operator-sdk run bundle
	$(OPERATOR_SDK) run bundle $(BUNDLE_IMG) --namespace declarest-system --install-mode AllNamespaces

catalog-build: opm ## Build the file-based catalog image ($(CATALOG_IMG))
	$(BUNDLE_IMAGE_BUILDER) build -f catalog.Dockerfile -t $(CATALOG_IMG) .

catalog-push: ## Push the file-based catalog image ($(CATALOG_IMG))
	$(BUNDLE_IMAGE_BUILDER) push $(CATALOG_IMG)

catalog: ## Regenerate the file-based OLM catalog for VERSION
	@mkdir -p catalog/declarest-operator
	@printf '%s\n' \
		'---' \
		'schema: olm.package' \
		'name: declarest-operator' \
		'defaultChannel: alpha' \
		'---' \
		'schema: olm.channel' \
		'package: declarest-operator' \
		'name: alpha' \
		'entries:' \
		'  - name: declarest-operator.v$(VERSION)' \
		'---' \
		'schema: olm.bundle' \
		'name: declarest-operator.v$(VERSION)' \
		'package: declarest-operator' \
		'image: $(BUNDLE_IMG)' \
		'properties:' \
		'  - type: olm.package' \
		'    value:' \
		'      packageName: declarest-operator' \
		'      version: $(VERSION)' \
		> catalog/declarest-operator/catalog.yaml

catalog-validate: opm ## Validate the file-based catalog under catalog/
	$(OPM) validate catalog

olm-install: ## Apply the OLM CatalogSource, OperatorGroup, and Subscription samples
	kubectl apply -k config/olm

olm-uninstall: ## Remove the OLM CatalogSource, OperatorGroup, and Subscription samples
	kubectl delete -k config/olm --ignore-not-found

verify-bundle: ## Regenerate the bundle and fail if bundle artifacts drift
	@before="$$(mktemp)"; \
	after="$$(mktemp)"; \
	trap 'rm -f "$$before" "$$after"' EXIT; \
	git diff -- bundle/ catalog/ config/manifests/ > "$$before"; \
	$(MAKE) release-bundle VERSION=$(VERSION); \
	git diff -- bundle/ catalog/ config/manifests/ > "$$after"; \
	if ! cmp -s "$$before" "$$after"; then \
		echo "ERROR: Bundle artifacts are out of date. Run 'make release-bundle' and commit the result."; \
		git diff --stat -- bundle/ catalog/ config/manifests/; \
		exit 1; \
	fi
