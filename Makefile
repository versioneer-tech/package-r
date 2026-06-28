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
	$Q cd frontend && pnpm install --frozen-lockfile && pnpm exec playwright test --project=chromium

.PHONY: test-backend
test-backend: ## Run backend tests
	$Q $(go) test -v ./...

.PHONY: test-usecases
test-usecases: ## Run executable documentation use cases
	$Q python3 scripts/test_usecases.py

.PHONY: test-e2e-s3
test-e2e-s3: ## Run local S3-compatible e2e checks
	$Q scripts/e2e-s3.sh

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
		if (/^[a-zA-Z_-]+:.*?##.*$$/) {printf "    ${YELLOW}%-20s${GREEN}%s${RESET}\n", $$1, $$2} \
		else if (/^## .*$$/) {printf "  ${CYAN}%s${RESET}\n", substr($$1,4)} \
		}' $(MAKEFILE_LIST)
