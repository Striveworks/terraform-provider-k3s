.ONESHELL:

default: fmt lint install generate


##@ Targets
gobincheck:
	if [ "$$(go env GOBIN)" != "$$HOME/go/bin" ]; then \
		echo "\033[0;31mERROR: Ensure your gobin is set to \$$HOME/go/bin\033[0m"; \
	fi

pre-commit-install:
	pre-commit install; \
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.1.6; \
	go install mvdan.cc/gofumpt@latest;

configure: gobincheck pre-commit-install ## Configures local terraform to use the binary
		cat <<EOF > "$$HOME/.terraformrc"
	provider_installation {
		dev_overrides {
			"striveworks.us/openstack/k3s" = "$$HOME/go/bin"
		}
		direct {}
	}
	EOF

vendor: ## Vendors the K3s script for offline installs
	curl https://get.k3s.io -o assets/k3s-install.sh

build: ## Builds the binary
	go build -v ./...

install: build ## Install locally the plugin
	go install -v ./...

lint: ## Lints the entire repo
	golangci-lint run

generate: ## Generates plugin docs. WARNING Only target requiring terraform and not opentofu
	cd tools; go generate ./...

fmt: ## Runs go formats
	gofmt -s -w -e .

test: ## Runs go tests
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc: ## Runs go acceptence tests e
	TF_ACC=1 go test -v -cover -timeout 120m ./...

help:  ## Display this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)


.PHONY: fmt lint test testacc build install generate help

# Use WARN for now so we can filter out noise from other providers
.PHONY: apply
apply:
	TF_LOG_PROVIDER=WARN terraform -chdir=examples/openstack apply -auto-approve

.PHONY: destroy
destroy:
	TF_LOG_PROVIDER=WARN terraform -chdir=examples/openstack destroy -auto-approve
