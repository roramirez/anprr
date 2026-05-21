BINARY := anprr
MODULE  := github.com/roramirez/anprr
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: all build install clean test lint vet fmt check run verify vuln help

all: help

help:
	@echo "Usage: make <target>"
	@echo ""
	@echo "Targets:"
	@echo "  build    Compile the binary"
	@echo "  install  Install the binary with go install"
	@echo "  run      Run the app without installing"
	@echo "  test     Run tests with race detection"
	@echo "  vet      Run go vet"
	@echo "  fmt      Format source files with gofmt"
	@echo "  lint     Run golangci-lint (implies vet)"
	@echo "  verify   Verify module dependencies"
	@echo "  vuln     Check for known vulnerabilities"
	@echo "  check    Run fmt, vet, verify, and test"
	@echo "  clean    Remove built binary"

build:
	go build $(LDFLAGS) -o $(BINARY) .

install:
	go install $(LDFLAGS) .

run:
	go run $(LDFLAGS) .

test:
	go test -race ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

lint: vet
	golangci-lint run ./...

verify:
	go mod verify

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

check: fmt vet verify test

clean:
	rm -f $(BINARY)
