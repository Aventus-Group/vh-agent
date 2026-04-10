# vh-agent

Resident Go daemon deployed inside every VibHost LXC container.
Communicates with `vh-provisioner-service` over gRPC (mTLS) to report health,
receive configuration updates, and stream real-time deploy logs.

**Phase 1 MVP — gRPC v2 scaffold** (branch `feature/grpc-v2-scaffold`).

---

## Architecture

```
LXC container
└── vh-agent  (this binary)
      ├── gRPC client  →  vh-provisioner AgentService
      │     ├── Heartbeat (unary, every N sec)
      │     ├── StreamLogs (server-streaming)
      │     └── GetConfig (unary)
      └── systemd unit  vibhost-agent.service
```

Transport: gRPC over WireGuard (`vhnet0`) private network `10.10.0.0/16`.
Auth: mTLS — provisioner CA signs both server cert and per-container client cert.

---

## Build

```bash
make proto   # generate Go from .proto (requires protoc + plugins)
make build   # cross-compile Linux amd64 static binary → ./vh-agent
```

Requires Go 1.24+.

---

## Test

```bash
make test    # go test ./... -race -cover
make lint    # golangci-lint run
make vet     # go vet ./...
```

---

## Configuration

The agent reads `/etc/vibhost/agent.conf` (env-file format) and `/etc/vibhost/agent.token`.

| Key            | Description                          |
|----------------|--------------------------------------|
| `CONTAINER_ID` | Unique container identifier          |
| `PROVISIONER_GRPC_ADDR` | `host:port` of AgentService gRPC endpoint |
| `CA_CERT`      | Path to provisioner CA certificate   |
| `CLIENT_CERT`  | Path to container client certificate |
| `CLIENT_KEY`   | Path to container client private key |

---

## Directory layout

```
vh-agent/
├── cmd/vh-agent/       main entry point
├── internal/
│   ├── config/         config loading (env-file + flags)
│   ├── agent/          gRPC client loop, heartbeat, log streaming
│   ├── metrics/        CPU / mem / disk / WireGuard collectors
│   └── pb/             generated protobuf (git-ignored, produced by make proto)
├── proto/
│   └── agent/v2/       agent.proto definition
├── systemd/            vibhost-agent.service unit
└── .github/workflows/  CI + release pipelines
```

---

## Status

v1 (HTTP polling) lives on `main`.
This branch (`feature/grpc-v2-scaffold`) is the Phase 1 MVP of the gRPC v2 rewrite.
See design doc: `vh-provisioner-service/docs/superpowers/specs/2026-04-10-vh-agent-design.md`.
