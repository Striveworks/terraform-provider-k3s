.ONESHELL:

##@ Targets

.PHONY: gobincheck
gobincheck:
	if [ "$$(go env GOBIN)" != "$$HOME/go/bin" ]; then \
		echo "\033[0;31mERROR: Ensure your gobin is set to \$$HOME/go/bin\033[0m"; \
	fi

.PHONY: pre-commit-install
pre-commit-install:
	pre-commit install; \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.1.6;

.PHONY: cfg-tfrc
cfg-tfrc:
	cat <<EOF > "$$HOME/.terraformrc"
	provider_installation {
		dev_overrides {
			"striveworks.us/openstack/k3s" = "$$HOME/go/bin"
		}
		direct {}
	}
	EOF

.PHONY: configure
configure: cfg-tfrc gobincheck pre-commit-install ## Configures local terraform to use the binary

.PHONY: vendor
vendor: ## Vendors the K3s script for offline installs
	curl https://get.k3s.io -o assets/k3s-install.sh

.PHONY: build
build: ## Builds the binary
	go build -v ./...

.PHONY: install
install: build ## Install locally the plugin
	go install -v ./...

.PHONY: lint
lint: ## Lints the entire repo
	golangci-lint run

.PHONY: generate
generate: ## Generates plugin docs. WARNING Only target requiring terraform and not opentofu
	cd tools; go generate ./...

.PHONY: fmt
fmt: ## Runs go formats
	gofmt -s -w -e .

.PHONY: test
test: ## Runs go tests
	go test -v -cover -timeout=120s -parallel=10 ./...

.PHONY: testacc
testacc: ## Runs go acceptence tests
	TF_ACC=1 go test -v -cover -timeout 120m ./...

.PHONY: apply # Use WARN for now so we can filter out noise from other providers
apply: ## Stands up the openstack example provider
	TF_LOG_PROVIDER=WARN tofu -chdir=examples/openstack apply -auto-approve

.PHONY: destroy
destroy: ## Use WARN for now so we can filter out noise from other providers
	TF_LOG_PROVIDER=WARN tofu -chdir=examples/openstack destroy -auto-approve

.PHONY: help
help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)
