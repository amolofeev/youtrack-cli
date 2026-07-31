GO ?= go

VERSION := $(shell cat ../VERSION)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILT   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X github.com/amolofeev/prompt-and-pray/internal/version.Version=$(VERSION) \
           -X github.com/amolofeev/prompt-and-pray/internal/version.Commit=$(COMMIT) \
           -X github.com/amolofeev/prompt-and-pray/internal/version.Built=$(BUILT)

.PHONY: build test lint vet integration

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/yt ./cmd/yt

test:
	$(GO) test ./...

lint:
	golangci-lint run

vet:
	$(GO) vet ./...

integration:
	YT_INTEGRATION=1 $(GO) test ./...
