# vh-agent

Resident gRPC daemon that runs inside every VibHost LXC container. Provides exec/file operations for remote deployments initiated by `vh-ai-agent` (Claude-driven deploy orchestrator), with live stdout/stderr streaming via gRPC bidi streams.

## Architecture

See [design spec](https://github.com/Aventus-Group/vh-provisioner-service/blob/main/docs/superpowers/specs/2026-04-10-vh-agent-v2-grpc-design.md) for the full rationale and API contract.

Phase 1 scope: exec, file (read/write/list), kill, health.
Phase 2 scope (not yet implemented): heartbeat, metrics, wg hot-patch, self-update.

## Build

```bash
make build       # produces ./vh-agent-linux-amd64
make test        # runs unit tests with -race
make lint        # golangci-lint
make proto-gen   # regenerate gen/agentpb/ from proto/agent.proto
```

## Run

```bash
# config lives at /etc/vibhost/agent.conf
# example contents:
#   GRPC_LISTEN_ADDR=10.10.75.42:50051
#   WORKDIR_DEFAULT=/home/appuser
#   LOG_LEVEL=info
sudo /usr/local/bin/vh-agent
```

Under systemd, see `systemd/vibhost-agent.service`.

## License

Internal — Aventus Group.
