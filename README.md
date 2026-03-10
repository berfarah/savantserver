# Savant MQTT Bridge

An MQTT bridge that connects a Savant SHR-2000 smart home system to Home
Assistant. Runs on the Savant host (ARM Linux) and translates between Savant's
avc WebSocket protocol and MQTT for HA auto-discovery and control.

## How it works

The bridge connects to three interfaces on the Savant host:

- **REST API** (`http://127.0.0.1:3062`) — discovers rooms, loads, and devices;
  reads initial state
- **avc WebSocket** (`ws://127.0.0.1:8480`) — sends dimmer/switch commands and
  receives real-time state updates
- **MQTT** (`tcp://<broker>:1883`) — publishes HA discovery configs, state
  updates, and subscribes to commands

Lights appear automatically in Home Assistant with correct room assignments,
dimmer support, and natural light mode handling.

## Setup

1. Edit `config.yaml` with your MQTT broker address (and optionally set
   `savant.config_name` for per-load state hydration)
2. Build: `./build`
3. Copy the binary and `config.yaml` to the Savant host
4. Copy `lib/savant-server.service` to `/lib/systemd/system/savant-server.service`
5. `sudo systemctl daemon-reload`
6. `sudo systemctl enable savant-server`
7. `sudo systemctl start savant-server`

Set `MQTT_PASSWORD` as an environment variable if your broker requires auth.

## Documentation

See `docs/` for reverse engineering findings and the design spec:

- [System Architecture](docs/01-savant-architecture.md)
- [REST API](docs/02-rest-api.md)
- [avc WebSocket Protocol](docs/04-avc-websocket.md)
- [Design Spec](docs/06-design.md)
