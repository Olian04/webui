.PHONY: format lint test help generate monaco build run

# Trees this library owns. A missing path fails the whole target.
SOURCE_CODE ?= ./cmd/... ./internal/... ./pkg/... ./test/...

# The linters (golangci-lint, govulncheck) are in tools/go.mod, a module of their own that is never published, so that go.mod of the library lists what a program
# that uses it needs and not the two hundred modules of a linter. They run on this module's code all the same. templ stays in go.mod: its runtime is a requirement
# of the library, and its generator, which is a few modules more, is the same version by being the same requirement.
TOOL ?= go tool -modfile=tools/go.mod

help: ## Show available make targets
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_.-]+:.*##/ {printf "%-24s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

format: ## Run go fmt and gofmt
	go fmt ./...
	gofmt -w .

lint: generate ## Run go vet, module verify, vuln scan, golangci
	go vet ./...
	go mod verify
	$(TOOL) govulncheck $(SOURCE_CODE)
	$(TOOL) golangci-lint run $(SOURCE_CODE)

test: generate ## Run tests
	go test -race -shuffle=on -timeout 180s $(SOURCE_CODE)

generate: ## Generate the application
	go tool templ generate ./...

monaco: ## Rebuild the embedded Monaco editor assets (needs node); the output is committed
	cd tools/monaco && npm ci && node build.mjs

build: generate ## Build the application
	go build -o webui ./cmd/demo

run: build ## Run the application
	./webui
