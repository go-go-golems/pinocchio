.PHONY: gifs

all: gifs

VERSION=v0.1.14

TAPES=$(shell ls doc/vhs/*tape)
gifs: $(TAPES)
	for i in $(TAPES); do vhs < $$i; done

docker-lint:
	docker run --rm -v $(shell pwd):/app -w /app golangci/golangci-lint:v1.50.1 golangci-lint run -v

lint:
	golangci-lint run -v

test:
	go test ./...

build:
	go generate ./...
	go build ./...

goreleaser:
	goreleaser release --skip=sign --snapshot --clean

tag-major:
	git tag $(shell svu major)

tag-minor:
	git tag $(shell svu minor)

tag-patch:
	git tag $(shell svu patch)

release:
	git push --tags
	GOPROXY=proxy.golang.org go list -m github.com/go-go-golems/pinocchio@$(shell svu current)

bump-glazed:
	go get github.com/go-go-golems/glazed@latest
	go get github.com/go-go-golems/clay@latest
	go get github.com/go-go-golems/parka@latest
	go get github.com/go-go-golems/bobatea@latest
	go get github.com/go-go-golems/geppetto@latest
	go get github.com/go-go-golems/prompto@latest
	go mod tidy

pinocchio_BINARY=$(shell which pinocchio)
install:
	go build -o ./dist/pinocchio ./cmd/pinocchio && \
		cp ./dist/pinocchio $(pinocchio_BINARY)
