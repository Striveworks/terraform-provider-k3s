.ONESHELL:
SHELL := /bin/bash

##@ Targets

.PHONY: gobincheck
gobincheck:
	if [ "$$(go env GOBIN)" != "$$HOME/go/bin" ]; then \
		echo "\033[0;31mERROR: Ensure your gobin is set to \$$HOME/go/bin\033[0m"; \
	fi

.PHONY: pre-commit-install
pre-commit-install:
	pre-commit install; \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin;

.PHONY: cfg-tfrc
cfg-tfrc:
	cat <<EOF > "$$HOME/.terraformrc"
	provider_installation {
		dev_overrides {
			"striveworks/k3s" = "$$HOME/go/bin"
		}
		direct {}
	}
	EOF

.PHONY: configure
configure: cfg-tfrc gobincheck pre-commit-install ## Configures local terraform to use the binary

.PHONY: vendor
vendor: ## Vendors the K3s script for offline installs
	curl https://get.k3s.io -o internal/k3s/assets/k3s-install.sh

.PHONY: build
build: ## Builds the binary
	go build -v ./...

.PHONY: install
install: build ## Install locally the plugin
	go install -v ./...

.PHONY: generate
generate: ## Generates plugin docs. WARNING Only target requiring terraform and not opentofu
	cd tools; go generate ./...

.PHONY: fmt
fmt: ## Runs go formats
	gofmt -s -w -e .

.PHONY: test
test:
	go test ./internal/provider -timeout 20m -v

.PHONY: test-acc
test-acc:
	TF_ACC=1 go test ./internal/provider -timeout 20m -v

.PHONY: help
help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
