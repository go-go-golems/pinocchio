SHELL := /usr/bin/env bash

.PHONY: all build web-install web-build generate dev clean

all: build

web-install:
	cd web && npm ci

web-build: web-install
	cd web && npm run build

generate:
	go generate ./...

build: web-build
	go build ./...

dev:
	cd web && npm run dev

clean:
	rm -rf static/dist

