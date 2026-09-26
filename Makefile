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
	$(go) build -ldflags '$(LDFLAGS)' -o package-r

.PHONY: build-backend-dev
build-backend-dev: ## Build backend with filesystem frontend assets for local/dev harnesses
	$Q CGO_ENABLED=1 \
	$(go) build -tags dev -ldflags '$(LDFLAGS)' -o package-r

# ------------------------------------------------------------------------------
# Test Targets
# ------------------------------------------------------------------------------

.PHONY: test
test: ## Check prerequisites and run all tests
	$Q bash tests/check-tools.bash
	$Q $(MAKE) --no-print-directory test-unit
	$Q $(MAKE) --no-print-directory test-integration
	$Q $(MAKE) --no-print-directory test-frontend

.PHONY: test-check
test-check: ## Check tools required by the full test suite
	$Q bash tests/check-tools.bash

.PHONY: test-unit
test-unit: ## Run Go unit tests
	$Q $(go) test -v ./...

.PHONY: test-integration
test-integration: ## Run API integration tests with local rclone S3
	$Q if command -v stac-check >/dev/null 2>&1; then \
		tests/integration/run.bash; \
	else \
		uv run --no-project --with-requirements tests/requirements.txt \
			tests/integration/run.bash; \
	fi

.PHONY: test-frontend
test-frontend: ## Run frontend Playwright tests
	$Q $(MAKE) build-backend-dev
	$Q cd frontend && pnpm install --frozen-lockfile && \
		PACKAGE_R_PLAYWRIGHT_BUILD=false pnpm exec playwright test --project=chromium

.PHONY: docs
docs: ## Build documentation with strict validation
	$Q uv run mkdocs build --strict

# ------------------------------------------------------------------------------
# Lint Targets
# ------------------------------------------------------------------------------

.PHONY: lint
lint: lint-frontend lint-backend ## Run all linters

.PHONY: lint-frontend
lint-frontend: ## Run frontend linters
	$Q cd frontend && pnpm install --frozen-lockfile && pnpm run lint && pnpm run typecheck

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
