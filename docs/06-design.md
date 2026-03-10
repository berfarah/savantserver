# MQTT Bridge Design

## Problem

The Savant SHR-2000 smart host has no native Home Assistant integration. Savant's ecosystem is closed — the only control surfaces are Savant's own apps, keypads, and web UI. This makes it impossible to use Google Home / Alexa voice control, build cross-system automations in HA, or have a unified dashboard.

## Goals

1. **Per-load light entities in HA** — every physical dimmer/switch appears as a controllable light, grouped by room
2. **Natural light preservation** — "turn on" activates the daylight curve; explicit brightness bypasses it
3. **Real-time state sync** — physical button presses reflect in HA within ~100ms
4. **Automatic discovery** — entities auto-appear in HA via MQTT Discovery, no YAML config
5. **Optimistic UI** — HA/Google Home responds instantly to commands, corrected if needed
6. **Resilient connections** — periodic WebSocket reconnection to mitigate potential memory leaks

## Non-Goals (v1)

- Scene management (user builds HA scenes/automations directly)
- Physical button press events as HA triggers
- Power/energy sensors, ambient light sensors
- Global system health monitoring

## Architecture

```
┌───────────────────────────────────────────────────────┐
│                 Savant Host (ARM Linux)                 │
│                                                         │
│  ┌───────────────────────────────────────────────────┐  │
│  │            savantserver (this binary)               │  │
│  │                                                   │  │
│  │  ┌──────────────────────────────────────────────┐ │  │
│  │  │            MQTT Bridge                       │ │  │
│  │  │                                              │ │  │
│  │  │  ┌────────────────────┐                      │ │  │
│  │  │  │   avc WS Client    │ subscribe + control  │ │  │
│  │  │  └─────────┬──────────┘                      │ │  │
│  │  │            │                                  │ │  │
│  │  │  ┌─────────┴──────────┐                      │ │  │
│  │  │  │  State Cache +     │                      │ │  │
│  │  │  │  Entity Registry   │                      │ │  │
│  │  │  └─────────┬──────────┘                      │ │  │
│  │  │            │                                  │ │  │
│  │  │  ┌─────────┴──────────┐                      │ │  │
│  │  │  │   MQTT Client      │ paho.mqtt.golang     │ │  │
│  │  │  └─────────┬──────────┘                      │ │  │
│  │  └────────────┼──────────────────────────────────┘ │  │
│  └───────────────┼────────────────────────────────────┘  │
│                  │                                       │
│  ┌──────────┐  ┌┴───────┐                               │
│  │openapi-go│  │  avc   │                               │
│  │ :3062    │  │ :8480  │                               │
│  └──────────┘  └────────┘                               │
└──────────────────┬───────────────────────────────────────┘
                   │ MQTT
            ┌──────┴──────┐
            │ MQTT Broker │
            └──────┬──────┘
                   │
            ┌──────┴──────┐
            │    Home     │
            │  Assistant  │
            └─────────────┘
```

## Why avc WS + REST?

We evaluated three connection strategies:

| Option | Pros | Cons |
|--------|------|------|
| avc WS + REST | Single WS for subscribe + control; REST for structured discovery | Must translate hardware addrs to room names |
| openapi-go WS + avc WS | Room-level names; auto-hydration on subscribe | Two WS connections, no control on openapi-go |
| NNG IPC + avc WS | Fastest state reads (~1ms) | Requires go-mangos dependency; no push subscriptions |

**We chose avc WS + REST** because:
- A single WebSocket handles both subscriptions and control
- The REST API provides structured config data (rooms, loads, devices, buttons) with UUIDs and relationships — ideal for one-time discovery
- Address translation is a solved problem (computed at startup from discovery data)
- Fewer connections = fewer failure modes

See `docs/02-rest-api.md` and `docs/04-avc-websocket.md` for API details.

## Connection Roles

| Connection | Protocol | Direction | Purpose |
|-----------|----------|-----------|---------|
| avc WS | `ws://127.0.0.1:8480` (`savant_protocol`) | Read+Write | State subscriptions + hardware control |
| REST API | `http://127.0.0.1:3062/config/v1/*` | Read | Discovery + initial state hydration |
| MQTT | `tcp://<broker>:1883` | Read+Write | Publish state, receive commands, HA discovery |

## Lifecycle

```
bridge.Start(ctx)
  ├── discover()           — REST → rooms, loads, devices (with buttons)
  ├── hydrateState()       — REST → batch GET /states/{name} → populate cache
  ├── mqttConnect()        — connect with LWT
  ├── publishDiscovery()   — retained HA discovery configs
  ├── publishAvailability() — "online" to savant/status
  ├── publishAllStates()   — current state for all entities
  ├── subscribeCommands()  — savant/+/+/light/set → handleCommand()
  ├── connectAVC()         — WS connect + handshake + subscribe(module, scene)
  ├── go readLoop()        — state/update → translate → cache → publish
  └── go reconnectLoop()   — every 5 min: close, reconnect, re-subscribe
```

Shutdown: context cancellation → publish "offline" → close WS → disconnect MQTT.

Reconnect: every 5 minutes, close and reconnect the avc WS to mitigate potential memory leaks in Savant firmware. On failure, logs and retries at the next 5-minute tick.

## Entity Mapping

Each of the 40 lighting loads becomes a `light` entity in HA. The mapping is built at startup from REST discovery data:

```
REST room + load + device → LightEntity {
  UniqueID:       "savant_load_005_0"
  Name:           "Lights"          (from load.name)
  RoomName:       "Den"             (from room.name)
  HexAddr:        0x50000           (deviceAddr << 16 | offset)
  IsDimmable:     true              (load.type == 0)
  FollowDaylight: true              (load.followDaylight)
  DaylightToggleAddr: 0x50002       (from button with "naturallighttogglewithoff" function)
}
```

### HA Discovery Payload

```json
{
  "schema": "json",
  "name": "Lights",
  "unique_id": "savant_load_005_0",
  "brightness": true,
  "brightness_scale": 100,
  "state_topic": "savant/den/lights/light/state",
  "command_topic": "savant/den/lights/light/set",
  "availability_topic": "savant/status",
  "device": {
    "identifiers": ["savant_load_005_0"],
    "name": "Den Lights",
    "manufacturer": "Savant",
    "model": "ECHO Adaptive phase",
    "suggested_area": "Den"
  }
}
```

## Command Routing

| HA Command | Bridge Action | Why |
|-----------|--------------|-----|
| `{"state":"ON"}` (no brightness) + followDaylight | Simulate NL toggle button (press + 200ms + release) | Respects daylight curve |
| `{"state":"ON"}` (no brightness) + !followDaylight | `load.<addr>` = `<max>%.0` | No curve to respect |
| `{"state":"ON","brightness":X}` | `load.<addr>` = `<X>%.0` | Explicit level bypasses curve |
| `{"state":"OFF"}` | `load.<addr>` = `0%.1` | Toggle off |
| `{"brightness":X}` (no state) | `load.<addr>` = `<X>%.0` | Adjust without toggling |

See `docs/appendix-a-natural-light.md` for how the daylight curve works.

## State Updates

When the avc WS pushes `module.<device_hex_addr>` with comma-separated load levels:

1. Parse device address from the state name
2. Split levels by comma — one per load offset
3. Skip `X` (no change) and `-1` (unknown)
4. Look up entity by `(deviceAddr << 16) | offset` in the address index
5. Update brightness + on/off in state cache
6. Publish to MQTT

## Optimistic Writes

When HA sends a command:

1. Update local state cache immediately
2. Publish updated state to MQTT (HA UI responds instantly)
3. Send command to avc WebSocket
4. When the avc WS pushes the actual value, publish it — if it differs, the correction propagates naturally

For `followDaylight` loads receiving ON (no brightness), the optimistic state publishes `{"state":"ON"}` without a brightness value, since the actual curve level is unknown. The real brightness arrives via the next avc WS `module` update (typically within ~100ms).

## MQTT Topics

```
savant/status                                    → "online" / "offline" (LWT)
savant/<room_slug>/<load_slug>/light/state       → {"state":"ON","brightness":75}
savant/<room_slug>/<load_slug>/light/set         ← {"state":"ON","brightness":50}
```

Slug rules: lowercase, spaces → `_`, apostrophes removed. `Evelyn's Room` → `evelyns_room`.

## Configuration

YAML config file (default `config.yaml`, override with `-config`):

```yaml
savant:
  host: "127.0.0.1"
  rest_port: 3062
  avc_port: 8480
  config_name: "Farah SEA"  # Savant configuration name, enables per-load state hydration

mqtt:
  broker: "tcp://192.168.4.10:1883"
  username: ""
  password: ""              # or MQTT_PASSWORD env var
  topic_prefix: "savant"

homeassistant:
  discovery_prefix: "homeassistant"
```

When `config_name` is set, state hydration fetches per-load dimmer levels using `<config_name>.RacePointMedia_host.CurrentDimmerLevel_1_<deviceAddr>`. Without it, falls back to room-level brightness (shared across all loads in a room).

## Code Structure

| File | Purpose |
|------|---------|
| `main.go` | Entry point: flags, config, signal handling, bridge.Start() |
| `config.go` | YAML config loading with defaults and env var override |
| `types.go` | All shared types (REST models, bridge domain types, helpers) |
| `savantapi.go` | REST API client + entity builder (BuildEntities) |
| `avcws.go` | avc WebSocket client (connect, subscribe, send, read loop) |
| `mqtt.go` | MQTT client (publish, subscribe, LWT, HA discovery payloads) |
| `bridge.go` | Orchestrator (lifecycle, state cache, command routing, reconnect) |

## Future Work

- **Button press events**: Subscribe to keypad press events for custom HA automations
- **Power sensors**: Expose per-load energy data. State name format: `<ConfigName>.RacePointMedia_host.<Property>_1_<DeviceAddr>` where Property is `Watts`, `Milliamps`, `Volts`, or `TempCelsius`
- **Ambient light sensors**: Expose per-device ambient levels. State name format: `<ConfigName>.RacePointMedia_host.CurrentAmbientLevel_<DeviceAddr>`
- **Exponential backoff**: Add exponential backoff (1s → 2s → 4s → ... → 30s max) for avc WS reconnection failures, rather than waiting for the next 5-minute tick
- **NNG direct client**: Use go-mangos for ~1ms state reads if latency matters
- **openapi-go WS**: Could supplement avc WS for room-level subscriptions with auto-hydration
- **REST API auth**: Investigate 402 auth mechanism on PUT endpoints
