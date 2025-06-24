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

# goimports (via go install — safe and semver clean)
GOIMPORTS_VERSION := v0.34.0
goimports := $(TOOLS_BIN)/goimports
$(goimports):
	$Q GOBIN=$(abspath $(TOOLS_BIN)) go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION)

# golangci-lint (install via script due to v2 module incompatibility)
GOLANGCI_LINT_VERSION := v2.1.6
golangci-lint := $(TOOLS_BIN)/golangci-lint
$(golangci-lint):
	$Q curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh \
		| sh -s -- -b $(abspath $(TOOLS_BIN)) $(GOLANGCI_LINT_VERSION)

# ---------------------
# Composite Target
# ---------------------

.PHONY: tools
tools: $(goimports) $(golangci-lint)
