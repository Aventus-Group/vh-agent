.PHONY: build test lint vet fmt proto clean

BINARY   := vh-agent
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -ldflags "-s -w -X main.Version=$(VERSION)"
GOOS     := linux
GOARCH   := amd64

# Proto generation (requires protoc, protoc-gen-go, protoc-gen-go-grpc)
PROTO_DIR := proto/agent/v2
PB_OUT    := internal/pb

proto:
	mkdir -p $(PB_OUT)
	protoc \
		--proto_path=$(PROTO_DIR) \
		--go_out=$(PB_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PB_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/agent.proto

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
	rm -f $(BINARY) $(BINARY)-* coverage.out coverage.html
	rm -rf $(PB_OUT)
