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
test: ## Run every test suite, including race detection and Chromium
	$Q $(MAKE) --no-print-directory test-check
	$Q $(MAKE) --no-print-directory test-tooling
	$Q $(MAKE) --no-print-directory test-unit
	$Q $(MAKE) --no-print-directory test-race
	$Q $(MAKE) --no-print-directory test-integration
	$Q $(MAKE) --no-print-directory test-frontend

.PHONY: setup-tests
setup-tests: ## Install frontend test dependencies and download missing test data
	$Q cd frontend && pnpm install --frozen-lockfile && \
		if ! node scripts/check-playwright.mjs >/dev/null 2>&1; then \
			pnpm exec playwright install --with-deps chromium && node scripts/check-playwright.mjs; \
		fi
	$Q if [[ ! -f tests/data/vienna-s2l2a-26/vienna-s2l2a-26.parquet ]]; then \
		uv run --no-project --with-requirements tests/requirements.txt \
			python scripts/download_sentinel2.py; \
	fi

.PHONY: test-check
test-check: ## Check C/C++ libraries, tools, and Chromium required by make test
	$Q bash tests/check-tools.bash

.PHONY: test-unit
test-unit: ## Run Go unit tests
	$Q $(go) test -v ./...

.PHONY: test-race
test-race: ## Check shared HTTP, storage, authentication, and telemetry code for races
	$Q CGO_ENABLED=1 $(go) test -race ./auth ./diskcache ./http ./logging ./metrics ./rclonefs

.PHONY: performance
performance: ## Run the timed public presign workloads on an idle host
	$Q CGO_ENABLED=1 $(go) test -tags loadtest ./http -run '^TestPublicSharePresignConcurrentHierarchy$$' -count=1 -v

.PHONY: test-tooling
test-tooling: ## Check CI selection, secret scan boundaries, and release security gates
	$Q uv run --locked --group tooling python -m unittest discover -s scripts/tests -v

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
	$Q cd frontend && pnpm install --frozen-lockfile && node scripts/check-playwright.mjs
	$Q $(MAKE) build-backend-dev
	$Q cd frontend && PACKAGE_R_PLAYWRIGHT_BUILD=false pnpm exec playwright test --project=chromium $(PLAYWRIGHT_ARGS)

.PHONY: docs
docs: ## Build documentation with strict validation
	$Q uv run --locked openapi-spec-validator docs/reference-guides/openapi.yaml
	$Q uv run --locked mkdocs build --strict

.PHONY: candidate
candidate: build ## Build the binary, OCI image, and dependency scan inputs
	$Q mkdir -p dist/candidate
	$Q docker buildx build --pull --platform linux/amd64 --provenance=false --sbom=false \
		--label "org.opencontainers.image.name=package-r" \
		--label "org.opencontainers.image.created=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
		--label "org.opencontainers.image.source=https://github.com/versioneer-tech/package-r" \
		--label "org.opencontainers.image.revision=$$(git rev-parse HEAD)" \
		--label "org.opencontainers.image.version=$(VERSION)" \
		--output type=oci,dest=dist/candidate/image.tar .
	$Q uv run --locked --group tooling python scripts/security.py prepare \
		--source . --candidate dist/candidate --version '$(VERSION)' --commit "$$(git rev-parse HEAD)"

.PHONY: security
security: $(trivy) ## Scan the existing candidate and enforce the release policy
	$Q uv run --locked --group tooling python scripts/security.py scan \
		--candidate dist/candidate --reports build/security --trivy $(trivy)

.PHONY: security-verify
security-verify: ## Check that the candidate matches its successful scan
	$Q uv run --locked --group tooling python scripts/security.py verify \
		--candidate dist/candidate --reports build/security

.PHONY: vulncheck
vulncheck: $(govulncheck) ## Check Go source for calls to known vulnerable code
	$Q mkdir -p build/security
	$Q set -o pipefail; CGO_ENABLED=1 $(govulncheck) ./... 2>&1 | tee build/security/govulncheck.txt

.PHONY: security-rescan
security-rescan: $(trivy) ## Scan a published release and its dependency manifests
	$Q uv run --locked --group tooling python scripts/security.py rescan \
		--source build/release-source --image '$(RELEASE_IMAGE)' \
		--binary build/release-metadata/linux-amd64-package-r.tar.gz \
		--manifest build/release-metadata/security-manifest.json \
		--reports build/security --trivy $(trivy)

# ------------------------------------------------------------------------------
# Lint Targets
# ------------------------------------------------------------------------------

.PHONY: hooks
hooks: ## Install commit and push hooks
	$Q uv run --locked --group tooling pre-commit install

.PHONY: check
check: lint-files lint ## Run CI lint checks without tests or builds

.PHONY: lint-files
lint-files: ## Check file syntax, whitespace, Markdown, Dockerfiles, and secrets
	$Q uv run --locked --group tooling pre-commit run --all-files --hook-stage manual

.PHONY: lint
lint: lint-frontend lint-backend ## Run all linters

.PHONY: lint-frontend
lint-frontend: ## Run frontend linters
	$Q cd frontend && pnpm install --frozen-lockfile && pnpm run lint && pnpm run typecheck

.PHONY: lint-backend
lint-backend: fmt-check | $(golangci-lint) ## Check Go formatting and run backend linters
	$Q $(golangci-lint) config verify
	$Q $(golangci-lint) run -v

go_sources = git ls-files -z --cached --others --exclude-standard -- '*.go' ':!:vendor/**' | \
	while IFS= read -r -d '' source; do \
		if [[ -f "$$source" ]]; then printf '%s\0' "$$source"; fi; \
	done

.PHONY: fmt-check
fmt-check: $(goimports) ## Check Go formatting and imports
	$Q set -euo pipefail; \
	files="$$( $(go_sources) | xargs -0 $(goimports) -local $(MODULE) -l)"; \
	if [[ -n "$$files" ]]; then \
		printf 'Run make fmt to format these files:\n%s\n' "$$files"; \
		exit 1; \
	fi

.PHONY: fmt
fmt: $(goimports) ## Format Go source files
	$Q set -euo pipefail; \
	$(go_sources) | xargs -0 $(goimports) -local $(MODULE) -w

# ------------------------------------------------------------------------------
# Clean Targets
# ------------------------------------------------------------------------------

.PHONY: clean
clean: clean-tools clean-frontend ## Remove build outputs, generated docs, and test reports
	$Q rm -rf package-r bin build dist site docs/generated

.PHONY: clean-frontend
clean-frontend: ## Remove frontend build outputs and browser test reports
	$Q if [[ -d frontend/dist ]]; then \
		find frontend/dist -mindepth 1 -maxdepth 1 ! -name .gitkeep -exec rm -rf {} +; \
	fi
	$Q rm -rf frontend/test-results frontend/playwright-report frontend/playwright/.cache

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
