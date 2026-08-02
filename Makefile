GO ?= go
GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || echo "$(HOME)/go/bin/golangci-lint")
TESTFLAGS ?= -cover -bench=. -count=1 -v

VERSION := $(shell cat VERSION)
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILT   := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -X github.com/amolofeev/youtrack-cli/internal/version.Version=$(VERSION) \
           -X github.com/amolofeev/youtrack-cli/internal/version.Commit=$(COMMIT) \
           -X github.com/amolofeev/youtrack-cli/internal/version.Built=$(BUILT)

.PHONY: build test lint vet integration

build:
	$(GO) build -trimpath -ldflags "$(LDFLAGS)" -o bin/yt ./cmd/yt

test:
	$(GO) test $(TESTFLAGS) ./...

lint:
	$(GOLANGCI_LINT) run

vet:
	$(GO) vet ./...

integration:
	YT_INTEGRATION=1 $(GO) test ./...
