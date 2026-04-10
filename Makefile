.PHONY: build test lint proto-gen clean

BINARY := vh-agent-linux-amd64
VERSION ?= dev
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION)"

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY) ./cmd/vh-agent/

test:
	go test ./... -race -coverprofile=coverage.out

lint:
	golangci-lint run

proto-gen:
	protoc \
	  --proto_path=proto \
	  --go_out=gen/agentpb --go_opt=paths=source_relative \
	  --go-grpc_out=gen/agentpb --go-grpc_opt=paths=source_relative \
	  proto/agent.proto

clean:
	rm -f $(BINARY) coverage.out
