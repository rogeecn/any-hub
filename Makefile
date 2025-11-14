GO       ?= /home/rogee/.local/go/bin/go
GOCACHE  ?= /tmp/go-build

.PHONY: build fmt test test-all run

build:
	$(GO) build ./cmd/any-hub

fmt:
	$(GO)fmt ./cmd ./internal ./tests

test:
	$(GO) test ./...

test-all:
	GOCACHE=$(GOCACHE) $(GO) test ./...

run:
	$(GO) run ./cmd/any-hub --config ./config.toml