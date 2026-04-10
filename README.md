# vh-agent

Resident Go daemon for VibHost LXC containers. Sends heartbeats, syncs WireGuard config, and self-updates.

**Status:** MVP implementation — see `vh-provisioner-service/docs/superpowers/specs/2026-04-10-vh-agent-design.md`.

## Build

```bash
make build
```

Produces `vh-agent` binary (Linux amd64, static, stripped).

## Run

```bash
VH_AGENT_CONFIG=/etc/vibhost/agent.conf \
VH_AGENT_TOKEN_FILE=/etc/vibhost/agent.token \
./vh-agent
```

## Tests

```bash
make test
make lint
```
