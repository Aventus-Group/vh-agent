.PHONY: build test lint clean vet fmt

BINARY := vh-agent
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"
GOOS := linux
GOARCH := amd64

build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build $(LDFLAGS) -o $(BINARY) ./cmd/vh-agent/
	@echo "Built $(BINARY) ($(VERSION)) — $$(du -h $(BINARY) | cut -f1)"

test:
	go test ./... -v -race -cover

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

clean:
	rm -f $(BINARY) coverage.out coverage.html

sha256: build
	sha256sum $(BINARY)
