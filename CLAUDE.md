# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build

Cross-compile for the Savant smart host (ARM Linux):
```bash
./build
```
This runs `env GOOS=linux GOARCH=arm GOARM=7 go build -v .`

Run tests:
```bash
go test ./...
```

## Architecture

This is an MQTT bridge that connects a Savant SHR-2000 smart home system to Home Assistant. It runs on the Savant host (ARM Linux) and translates between Savant's avc WebSocket protocol and MQTT for HA auto-discovery and control.

See `docs/06-design.md` for the full design spec.

**Key files:**
- `main.go` — Entry point: `-config` flag, signal handling, bridge.Start()
- `config.go` — YAML config loading with defaults and `MQTT_PASSWORD` env var override
- `types.go` — All shared types (REST API models, bridge domain types, slug/address helpers)
- `savantapi.go` — REST API client for discovery (rooms, loads, devices, buttons) + `BuildEntities` joins them into `LightEntity` values
- `avcws.go` — avc WebSocket client (port 8480, `savant_protocol` subprotocol): connect, handshake, subscribe, SetLoad, SimulateButtonPress, ReadLoop
- `mqtt.go` — MQTT client: HA discovery payloads, state publishing, command subscription, LWT
- `bridge.go` — Orchestrator: lifecycle (discover → hydrate → MQTT → avc), state cache, command routing, optimistic writes, periodic reconnect

**Config:** `config.yaml` (see `docs/06-design.md` for format). Key fields: `savant.config_name` enables per-load state hydration; `mqtt.broker` is required.

**Connections:**
- avc WebSocket (`ws://127.0.0.1:8480`): hardware control + real-time state subscriptions
- REST API (`http://127.0.0.1:3062`): config discovery + state hydration
- MQTT (`tcp://<broker>:1883`): HA integration (publish state, receive commands, auto-discovery)
