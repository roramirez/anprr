BINARY := anprr
MODULE  := github.com/roramirez/anprr
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: all build install clean test lint vet fmt check run verify vuln

all: build

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
