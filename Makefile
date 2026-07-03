.PHONY: all build test test-cover lint vet vuln fmt tidy clean check release-snapshot install help

BINARY    := cli_mate
MODULE    := ./cmd/cli_mate
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE      ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "unknown")
LDFLAGS   := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

## all: Default target — lint, test, build
all: lint test build

## build: Build the binary
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(MODULE)

## test: Run all tests with race detector
test:
	go test -race -count=1 ./...

## test-cover: Run tests with coverage report
test-cover:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	@echo "Coverage report: coverage.out"
	@go tool cover -func=coverage.out | tail -1

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## vet: Run go vet
vet:
	go vet ./...

## vuln: Run govulncheck
vuln:
	govulncheck ./...

## fmt: Format Go code
fmt:
	gofmt -s -w .

## tidy: Tidy and verify go.mod
tidy:
	go mod tidy
	go mod verify

## clean: Remove build artifacts
clean:
	rm -f $(BINARY) $(BINARY).exe coverage.out coverage.html

## check: Run all checks (vet, lint, test)
check: vet lint test

## release-snapshot: Build a local snapshot release (no publish)
release-snapshot:
	goreleaser release --snapshot --clean

## install: Install binary to GOPATH/bin
install:
	go install -ldflags "$(LDFLAGS)" $(MODULE)

## help: Show this help message
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
