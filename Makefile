SHELL := /bin/bash

.PHONY: fmt test build docs check

fmt:
	go fmt ./...
	terraform -chdir=examples/basic fmt

test:
	go test ./...

build:
	go build ./...

docs:
	go generate ./...

check: fmt test build
