.PHONY: all test build lint lintmax docker-lint golangci-lint-install gosec govulncheck goreleaser tag-major tag-minor tag-patch release bump-go-go-golems install codeql-local geppetto-lint-build geppetto-lint glazed-lint-build glazed-lint web-typecheck web-lint web-check proto-gen proto-gen-core proto-gen-web-chat schema-vet fetch-spa clean-spa build-with-spa logcopter-generate logcopter-check

all: test build

VERSION=v0.1.14
GORELEASER_ARGS ?= --skip=sign --snapshot --clean
GORELEASER_TARGET ?= --single-target
GOLANGCI_LINT_VERSION ?= $(shell cat .golangci-lint-version)
GOLANGCI_LINT_BIN ?= $(CURDIR)/.bin/golangci-lint
SESSIONSTREAM_LINT ?= /tmp/sessionstream-lint
SESSIONSTREAM_LINT_PKG ?= ../sessionstream/cmd/sessionstream-lint

TAPES=$(shell ls doc/vhs/*tape 2>/dev/null || echo "")
gifs:
	for i in $(TAPES); do vhs < $$i; done

# Build geppetto-lint vettool from geppetto module
# Uses the version specified in go.mod
GEPPETTO_LINT_BIN ?= /tmp/geppetto-lint
GEPPETTO_LINT_PKG ?= github.com/go-go-golems/geppetto/cmd/tools/geppetto-lint
GEPPETTO_VERSION ?= $(shell go list -m -f '{{.Version}}' github.com/go-go-golems/geppetto 2>/dev/null)
GLAZED_LINT_BIN ?= /tmp/glazed-lint
GLAZED_LINT_PKG ?= github.com/go-go-golems/glazed/cmd/tools/glazed-lint
GLAZED_VERSION ?= $(shell GOWORK=off go list -m -f '{{.Version}}' github.com/go-go-golems/glazed 2>/dev/null)
GLAZED_LINT_FLAGS ?= -glazedclilint.allow-paths=pkg/analysis/,pkg/cli/,pkg/cmds/fields/,pkg/cmds/logging/,pkg/cmds/sources/,pkg/help/,pkg/cmds/cmdlayers/,cmd/pinocchio/cmds/clip.go,cmd/pinocchio/cmds/serve.go

geppetto-lint-build:
	@echo "Building geppetto-lint from geppetto module..."
	@# In CI, GOFLAGS often includes -mod=readonly; installing without @version can require adding go.sum entries.
	@# Installing with an explicit version avoids modifying the current module's go.{mod,sum}.
	@# In a go.work workspace, go list -m reports "(devel)", so we fall back to workspace install.
	@if [ -n "$(GEPPETTO_VERSION)" ] && [ "$(GEPPETTO_VERSION)" != "(devel)" ]; then \
		echo "Installing $(GEPPETTO_LINT_PKG)@$(GEPPETTO_VERSION)"; \
		GOBIN=$(dir $(GEPPETTO_LINT_BIN)) go install $(GEPPETTO_LINT_PKG)@$(GEPPETTO_VERSION); \
	else \
		echo "Installing $(GEPPETTO_LINT_PKG) from workspace/module"; \
		GOBIN=$(dir $(GEPPETTO_LINT_BIN)) go install $(GEPPETTO_LINT_PKG); \
	fi

geppetto-lint: geppetto-lint-build
	go vet -vettool=$(GEPPETTO_LINT_BIN) ./...

glazed-lint-build:
	@echo "Building glazed-lint from Glazed module..."
	@if [ -n "$(GLAZED_VERSION)" ] && [ "$(GLAZED_VERSION)" != "(devel)" ]; then \
		echo "Installing $(GLAZED_LINT_PKG)@$(GLAZED_VERSION)"; \
		GOBIN=$(dir $(GLAZED_LINT_BIN)) GOWORK=off go install $(GLAZED_LINT_PKG)@$(GLAZED_VERSION); \
	else \
		echo "Installing $(GLAZED_LINT_PKG) from workspace/module"; \
		GOBIN=$(dir $(GLAZED_LINT_BIN)) go install $(GLAZED_LINT_PKG); \
	fi

glazed-lint: glazed-lint-build
	go vet -vettool=$(GLAZED_LINT_BIN) $(GLAZED_LINT_FLAGS) ./cmd/... ./pkg/...

docker-lint:
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) golangci-lint run -v

golangci-lint-install:
	mkdir -p $(dir $(GOLANGCI_LINT_BIN))
	GOBIN=$(dir $(GOLANGCI_LINT_BIN)) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: build geppetto-lint-build glazed-lint-build golangci-lint-install
	$(GOLANGCI_LINT_BIN) run -v
	go vet -vettool=$(GEPPETTO_LINT_BIN) ./...
	go vet -vettool=$(GLAZED_LINT_BIN) $(GLAZED_LINT_FLAGS) ./cmd/... ./pkg/...

lintmax: build geppetto-lint-build glazed-lint-build golangci-lint-install
	$(GOLANGCI_LINT_BIN) run -v --max-same-issues=100
	go vet -vettool=$(GEPPETTO_LINT_BIN) ./...
	go vet -vettool=$(GLAZED_LINT_BIN) $(GLAZED_LINT_FLAGS) ./cmd/... ./pkg/...

gosec:
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -exclude=G101,G203,G304,G301,G306,G204,G302 -exclude-generated -exclude-dir=.history -exclude-dir=testdata -exclude-dir=pkg/chatapp/pb ./...

govulncheck:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

test:
	go test ./...

build:
	go generate ./...
	go build ./...

web-typecheck:
	cd cmd/web-chat/web && npm run typecheck

web-lint:
	cd cmd/web-chat/web && npm run lint

web-check: web-typecheck web-lint

proto-gen-core:
	buf generate --template buf.chatapp.gen.yaml --path proto/pinocchio
	buf generate --template buf.chatapp.web.gen.yaml --path proto/pinocchio

proto-gen: proto-gen-core

schema-vet:
	go build -o $(SESSIONSTREAM_LINT) $(SESSIONSTREAM_LINT_PKG)
	go vet -vettool=$(SESSIONSTREAM_LINT) ./cmd/... ./pkg/...

logcopter-generate:
	go generate ./...

logcopter-check:
	go tool logcopter-gen -area-prefix go-go-golems.pinocchio -strip-prefix github.com/go-go-golems/pinocchio -check ./pkg/... ./cmd/...

goreleaser:
	goreleaser release $(GORELEASER_ARGS) $(GORELEASER_TARGET)

tag-major:
	git tag $(shell svu major)

tag-minor:
	git tag $(shell svu minor)

tag-patch:
	git tag $(shell svu patch)

release:
	git push origin --tags
	GOPROXY=proxy.golang.org go list -m github.com/go-go-golems/pinocchio@$(shell svu current)

bump-go-go-golems:
	@deps="$$(awk '/^require[[:space:]]+github\.com\/go-go-golems\// { print $$2 } /^[[:space:]]*github\.com\/go-go-golems\// { print $$1 }' go.mod | sort -u)"; \
	if [ -z "$$deps" ]; then \
		echo "No github.com/go-go-golems dependencies in go.mod"; \
	else \
		echo "Bumping go-go-golems dependencies:"; \
		echo "$$deps"; \
		for dep in $$deps; do go get "$${dep}@latest"; done; \
	fi
	go mod tidy

# Path to CodeQL CLI - adjust based on installation location
CODEQL_PATH ?= $(shell which codeql)
# Path to CodeQL queries - adjust based on where you cloned the repository
CODEQL_QUERIES ?= $(HOME)/codeql-go/ql/src/go

# Create CodeQL database and run analysis
codeql-local:
	@if [ -z "$(CODEQL_PATH)" ]; then echo "CodeQL CLI not found. Install from https://github.com/github/codeql-cli-binaries/releases"; exit 1; fi
	@if [ ! -d "$(CODEQL_QUERIES)" ]; then echo "CodeQL queries not found. Clone from https://github.com/github/codeql-go"; exit 1; fi
	$(CODEQL_PATH) database create --language=go --source-root=. ./codeql-db
	$(CODEQL_PATH) database analyze ./codeql-db $(CODEQL_QUERIES)/Security --format=sarif-latest --output=codeql-results.sarif
	@echo "Results saved to codeql-results.sarif"

# SPA frontend assets from the glazed release.
# Downloads the help browser SPA and extracts it for embedding.
# Parses go.mod directly (go list doesn't work in workspace mode).
GLAZED_VERSION := $(shell grep 'go-go-golems/glazed ' go.mod | head -1 | awk '{print $$2}')
GLAZED_VERSION_NO_V := $(patsubst v%,%,$(GLAZED_VERSION))
GLAZED_SPA_DIR := pkg/spa/dist

fetch-spa:
	@if [ -z "$(GLAZED_VERSION)" ]; then echo "Warning: cannot detect glazed version from go.mod, skipping SPA fetch"; exit 0; fi
	@mkdir -p $(GLAZED_SPA_DIR)
	@echo "Fetching SPA assets for glazed $(GLAZED_VERSION)..."
	@curl -sfL https://github.com/go-go-golems/glazed/releases/download/$(GLAZED_VERSION)/glazed-spa-$(GLAZED_VERSION_NO_V).tar.gz \
		| tar xz -C $(GLAZED_SPA_DIR) \
	|| (echo "Warning: SPA assets not found for glazed $(GLAZED_VERSION), building without browser UI"; rm -rf $(GLAZED_SPA_DIR))

clean-spa:
	rm -rf $(GLAZED_SPA_DIR)

build-with-spa: fetch-spa
	go build -tags embed -o ./pinocchio ./cmd/pinocchio

pinocchio_BINARY=$(shell which pinocchio)
install:
	go build -o ./dist/pinocchio ./cmd/pinocchio && \
		cp ./dist/pinocchio $(pinocchio_BINARY)
