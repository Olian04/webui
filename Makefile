.PHONY: format lint test help generate assets monaco build run

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

ASSETS := internal/render/assets

generate: assets ## Generate the application: the templates, and the minified assets
	go tool templ generate ./...

# The stylesheets and scripts that the renderer serves are written in css/ and js/, as a person reads them, and served from min/, minified. The output is committed, as the
# editor's is, so that building the library needs no tool, and CI fails when it is not the output of the sources. The scripts are not bundled and not rewritten for an older
# engine: whitespace and local names only, and the names a classic script shares through the global scope are kept.
assets: ## Minify the stylesheets and scripts the renderer serves (sources in css/ and js/, output in min/, committed)
	rm -rf $(ASSETS)/min
	$(TOOL) esbuild $(ASSETS)/css/app.css $(ASSETS)/css/icons.css --minify --legal-comments=none --log-level=warning --outdir=$(ASSETS)/min/css
	$(TOOL) esbuild $(ASSETS)/js/*.js --minify --legal-comments=none --log-level=warning --outdir=$(ASSETS)/min/js
	cd tools && go run ./precompress ../$(ASSETS)/min

monaco: ## Rebuild the embedded Monaco editor assets (needs node); the output is committed
	cd tools/monaco && npm ci && node build.mjs
	cd tools && go run ./precompress ../pkg/monaco/editor ../pkg/monaco/css/worker ../pkg/monaco/html/worker ../pkg/monaco/json/worker ../pkg/monaco/typescript/worker

build: generate ## Build the application
	go build -o webui ./cmd/demo

run: build ## Run the application
	./webui
