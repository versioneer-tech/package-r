include common.mk
include tools.mk

LDFLAGS += -w -s \
	-X "$(MODULE)/version.Version=$(VERSION)" \
	-X "$(MODULE)/version.CommitSHA=$(VERSION_HASH)"

# ------------------------------------------------------------------------------
# Build Targets
# ------------------------------------------------------------------------------

.PHONY: build
build: | build-backend ## Build everything

.PHONY: build-frontend
build-frontend: ## Build frontend
	$Q cd frontend && pnpm install --frozen-lockfile && pnpm run build

.PHONY: build-backend
build-backend: | build-frontend ## Build backend and embed the current frontend bundle
	$Q CGO_ENABLED=1 \
	$(go) build -ldflags '$(LDFLAGS)' -o filebrowser

.PHONY: build-backend-dev
build-backend-dev: ## Build backend with filesystem frontend assets for local/dev harnesses
	$Q CGO_ENABLED=1 \
	$(go) build -tags dev -ldflags '$(LDFLAGS)' -o filebrowser

# ------------------------------------------------------------------------------
# Test Targets
# ------------------------------------------------------------------------------

.PHONY: test
test: | test-frontend test-backend test-usecases ## Run all tests

.PHONY: test-frontend
test-frontend: ## Run frontend tests
	$Q cd frontend && pnpm install --frozen-lockfile && pnpm run typecheck

.PHONY: test-frontend-e2e
test-frontend-e2e: ## Run frontend Playwright tests
	$Q $(MAKE) build-backend-dev
	$Q cd frontend && PACKAGE_R_PLAYWRIGHT_BUILD=false pnpm exec playwright test --project=chromium

.PHONY: test-backend
test-backend: ## Run backend tests
	$Q $(go) test -v ./...

.PHONY: test-usecases
test-usecases: ## Run executable documentation use cases
	$Q python3 scripts/test_usecases.py

.PHONY: test-production-build
test-production-build: ## Build production binary and smoke-check embedded frontend
	$Q bash scripts/production_smoke.sh

.PHONY: test-e2e-s3
test-e2e-s3: ## Run local S3-compatible e2e checks
	$Q scripts/e2e-s3.sh

.PHONY: test-e2e-s3-k8s
test-e2e-s3-k8s: ## Run Kubernetes/kind S3-compatible e2e checks
	$Q scripts/e2e-s3-k8s.sh

.PHONY: test-e2e-s3-k8s-keep
test-e2e-s3-k8s-keep: ## Run Kubernetes/kind S3 e2e and keep it open for manual inspection
	$Q PACKAGE_R_K8S_KEEP_CLUSTER=true \
		PACKAGE_R_K8S_KEEP_FORWARD=true \
		PACKAGE_R_FORWARD_PORT="$${PACKAGE_R_FORWARD_PORT:-8888}" \
		S3_FORWARD_PORT="$${S3_FORWARD_PORT:-19000}" \
		scripts/e2e-s3-k8s.sh

# ------------------------------------------------------------------------------
# Lint Targets
# ------------------------------------------------------------------------------

.PHONY: lint
lint: lint-frontend lint-backend ## Run all linters

.PHONY: lint-frontend
lint-frontend: ## Run frontend linters
	$Q cd frontend && pnpm install --frozen-lockfile && pnpm run lint

.PHONY: lint-backend
lint-backend: | $(golangci-lint) ## Run backend linters
	$Q $(golangci-lint) run -v

.PHONY: fmt
fmt: $(goimports) ## Format Go source files
	$Q $(goimports) -local $(MODULE) -w $$(find . -type f -name '*.go' -not -path "./vendor/*")

# ------------------------------------------------------------------------------
# Clean Targets
# ------------------------------------------------------------------------------

.PHONY: clean
clean: clean-tools ## Clean all build artifacts

# ------------------------------------------------------------------------------
# Help Target
# ------------------------------------------------------------------------------

.PHONY: help
help: ## Show this help
	@echo ''
	@echo 'Usage:'
	@echo '  ${YELLOW}make${RESET} ${GREEN}<target> [options]${RESET}'
	@echo ''
	@echo 'Options:'
	@$(call global_option, "V [0|1]", "enable verbose mode (default:0)")
	@echo ''
	@echo 'Targets:'
	@awk 'BEGIN {FS = ":.*?## "} { \
		if (/^[a-zA-Z0-9_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)
