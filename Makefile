.PHONY: all test build lint lintmax docker-lint gosec govulncheck goreleaser tag-major tag-minor tag-patch release bump-glazed install codeql-local geppetto-lint-build geppetto-lint

all: test build

VERSION=v0.1.14
GORELEASER_ARGS ?= --skip=sign --snapshot --clean
GORELEASER_TARGET ?= --single-target

TAPES=$(shell ls doc/vhs/*tape 2>/dev/null || echo "")
gifs:
	for i in $(TAPES); do vhs < $$i; done

# Build geppetto-lint vettool from geppetto module
# Uses the version specified in go.mod
GEPPETTO_LINT_BIN ?= /tmp/geppetto-lint
GEPPETTO_LINT_PKG ?= github.com/go-go-golems/geppetto/cmd/geppetto-lint
GEPPETTO_VERSION ?= $(shell go list -m -f '{{.Version}}' github.com/go-go-golems/geppetto 2>/dev/null)

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

docker-lint:
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:v2.4.0 golangci-lint run -v

lint: build geppetto-lint-build
	golangci-lint run -v
	go vet -vettool=$(GEPPETTO_LINT_BIN) ./...

lintmax: build geppetto-lint-build
	golangci-lint run -v --max-same-issues=100
	go vet -vettool=$(GEPPETTO_LINT_BIN) ./...

gosec:
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -exclude=G101,G304,G301,G306,G204,G302 -exclude-dir=.history -exclude-dir=testdata ./...

govulncheck:
	go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

test:
	go test ./...

build:
	go generate ./...
	go build ./...

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

bump-glazed:
	go get github.com/go-go-golems/glazed@latest
	go get github.com/go-go-golems/clay@latest
	go get github.com/go-go-golems/parka@latest
	go get github.com/go-go-golems/bobatea@latest
	go get github.com/go-go-golems/geppetto@latest
	go get github.com/go-go-golems/prompto@latest
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

pinocchio_BINARY=$(shell which pinocchio)
install:
	go build -o ./dist/pinocchio ./cmd/pinocchio && \
		cp ./dist/pinocchio $(pinocchio_BINARY)
