GO       ?= /home/rogee/.local/go/bin/go
GOCACHE  ?= /tmp/go-build

.PHONY: build fmt test test-all run

build:
	$(GO) build .

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

test-all:
	GOCACHE=$(GOCACHE) $(GO) test ./...

run:
	$(GO) run . --config ./config.toml
