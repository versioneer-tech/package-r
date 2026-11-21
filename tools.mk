include common.mk

TOOLS_DIR := $(BASE_PATH)/tools
TOOLS_BIN := $(TOOLS_DIR)/bin
$(eval $(shell mkdir -p $(TOOLS_BIN)))
PATH := $(TOOLS_BIN):$(PATH)
export PATH

.PHONY: clean-tools
clean-tools:
	$Q rm -rf $(TOOLS_BIN)

# ---------------------
# Go Tools
# ---------------------

GOIMPORTS_VERSION := v0.50.0
goimports := $(TOOLS_BIN)/goimports-$(GOIMPORTS_VERSION)
$(goimports):
	$Q GOBIN=$(abspath $(TOOLS_BIN)) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)
	$Q mv $(TOOLS_BIN)/goimports $@

# golangci-lint (via go install so the local Go toolchain builds the binary)
GOLANGCI_LINT_VERSION := v2.15.0
golangci-lint := $(TOOLS_BIN)/golangci-lint-$(GOLANGCI_LINT_VERSION)
$(golangci-lint):
	$Q GOBIN=$(abspath $(TOOLS_BIN)) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$Q mv $(TOOLS_BIN)/golangci-lint $@

GOVULNCHECK_VERSION := v1.10.0
govulncheck := $(TOOLS_BIN)/govulncheck-$(GOVULNCHECK_VERSION)
$(govulncheck):
	$Q GOBIN=$(abspath $(TOOLS_BIN)) go install golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION)
	$Q mv $(TOOLS_BIN)/govulncheck $@

# ---------------------
# Composite Target
# ---------------------

TRIVY_VERSION := 0.76.2
trivy := $(TOOLS_BIN)/trivy-$(TRIVY_VERSION)
$(trivy):
	$Q uv run --locked --group tooling python scripts/install_trivy.py $(TRIVY_VERSION) $@

.PHONY: tools
tools: $(goimports) $(golangci-lint) $(govulncheck)
