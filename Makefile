GO ?= go
TOOLS_DIR := hack/tools

GOLANGCI_LINT_VER := 2.12.0
GOLANGCI_LINT := $(TOOLS_DIR)/golangci-lint-$(GOLANGCI_LINT_VER)
GOLANGCI_LINT_FLAGS ?= --config $(realpath .golangci.yml)

KCP_VER := 0.31.0
KCP := $(TOOLS_DIR)/kcp-$(KCP_VER)

KUBECTL_VER := 1.35.1
KUBECTL := $(TOOLS_DIR)/kubectl-$(KUBECTL_VER)

.PHONY: check
check: lint test

.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run $(GOLANGCI_LINT_FLAGS) ./...
	cd test/e2e && $(CURDIR)/$(GOLANGCI_LINT) run $(GOLANGCI_LINT_FLAGS) ./...

.PHONY: lint-fix
lint-fix: override GOLANGCI_LINT_FLAGS := $(GOLANGCI_LINT_FLAGS) --fix
lint-fix: lint

.PHONY: test
test:
	$(GO) test -cover -race ./...

.PHONY: test-e2e
test-e2e: $(KCP) $(KUBECTL)
	cd test/e2e && TEST_ASSET_KCP=$(CURDIR)/$(KCP) TEST_ASSET_KUBECTL=$(CURDIR)/$(KUBECTL) $(GO) test -v -count=1 -race ./...

$(GOLANGCI_LINT):
	mkdir -p $(TOOLS_DIR)
	$(GO) tool github.com/ntnn/mindl download -common -out $@ -tool golangci-lint -version $(GOLANGCI_LINT_VER)

$(KCP):
	mkdir -p $(TOOLS_DIR)
	$(GO) tool github.com/ntnn/mindl download -out $@ \
		-url 'https://github.com/kcp-dev/kcp/releases/download/v{{.Version}}/kcp_{{.Version}}_{{.OS}}_{{.Arch}}.tar.gz' \
		-inarchive 'bin/kcp' \
		-version $(KCP_VER)

$(KUBECTL):
	mkdir -p $(TOOLS_DIR)
	$(GO) tool github.com/ntnn/mindl download -out $@ \
		-url 'https://dl.k8s.io/v{{.Version}}/kubernetes-client-{{.OS}}-{{.Arch}}.tar.gz' \
		-inarchive 'kubernetes/client/bin/kubectl' \
		-version $(KUBECTL_VER)
