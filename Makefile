# mlrshift Makefile
#
# Builds the single "mlrshift" binary from the repo-root main package and
# injects build-info via -ldflags into github.com/mnemoo/mlrshift/internal/cli.

BINARY      := mlrshift
PKG         := github.com/mnemoo/mlrshift
CLI_PKG     := $(PKG)/internal/cli
DIST        := dist

# Build metadata. These tolerate a repo with no tags or no commits yet.
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT      := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE        := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS     := -s -w \
	-X $(CLI_PKG).Version=$(VERSION) \
	-X $(CLI_PKG).Commit=$(COMMIT) \
	-X $(CLI_PKG).Date=$(DATE)

GO          := go
GOFLAGS     :=

# Cross-compile target matrix: GOOS/GOARCH pairs (all verified to build).
PLATFORMS := \
	windows/amd64 windows/386 windows/arm64 \
	linux/amd64 linux/386 linux/arm64 linux/arm \
	darwin/amd64 darwin/arm64 \
	freebsd/amd64

.DEFAULT_GOAL := help

.PHONY: help build install test test-race cover lint fmt vet tidy build-all run clean

## help: Show this help (default target).
help:
	@echo "mlrshift make targets:"
	@grep -E '^## [a-z-]+:' $(MAKEFILE_LIST) | sed -e 's/## /  /' | sort

## build: Build the mlrshift binary for the host platform.
build:
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o $(BINARY) .

## install: Install mlrshift into $GOBIN / $GOPATH/bin.
install:
	CGO_ENABLED=0 $(GO) install $(GOFLAGS) -ldflags '$(LDFLAGS)' .

## test: Run the test suite.
test:
	$(GO) test ./...

## test-race: Run the test suite with the race detector.
test-race:
	$(GO) test -race ./...

## cover: Run tests with coverage and print a per-function report.
cover:
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -func=coverage.out

## lint: Run golangci-lint over all packages.
lint:
	golangci-lint run ./...

## fmt: Format all Go source with gofmt.
fmt:
	gofmt -w .

## vet: Run go vet over all packages.
vet:
	$(GO) vet ./...

## tidy: Tidy go.mod (stdlib-only; should be a no-op).
tidy:
	$(GO) mod tidy

## build-all: Cross-compile every release target into ./dist.
build-all:
	@mkdir -p $(DIST)
	@set -e; for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
		out="$(DIST)/$(BINARY)_$${os}_$${arch}$${ext}"; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			$(GO) build $(GOFLAGS) -ldflags '$(LDFLAGS)' -o "$$out" . ; \
	done

## run: Build and run mlrshift (pass args via ARGS="...").
run: build
	./$(BINARY) $(ARGS)

## clean: Remove build artifacts.
clean:
	rm -rf $(DIST)
	rm -f $(BINARY) $(BINARY).exe coverage.out coverage*
