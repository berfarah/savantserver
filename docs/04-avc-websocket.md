# avc Lighting WebSocket

The primary hardware control API. Hosted by the `avc` process on port 8480, this WebSocket provides full read/write access to lighting hardware, scenes, rules, and device configuration. Used by Savant's web-based lighting app.

## Connection

```
URL:         ws://<host>:8480
Subprotocol: savant_protocol (REQUIRED)
Auth:        none required (optional session/authenticationRequest)
```

Port 8480 is **not proxied through nginx** — connect directly.

## Message Format

All messages use the same JSON envelope:

```json
{
  "URI": "<action-path>",
  "messages": [<payload-objects>]
}
```

## Session Lifecycle

### Handshake (required first message)

```json
{
  "URI": "session/devicePresent",
  "messages": [{
    "protocolVersion": "0.1",
    "device": {
      "name": "savantserver",
      "version": "1.0",
      "app": "savantserver",
      "ip": "127.0.0.1",
      "model": "server"
    }
  }]
}
```

Response:
```json
{
  "URI": "session/deviceRecognized",
  "messages": [{
    "controllerType": "...",
    "configID": "...",
    "hostUID": "...",
    "firmwareUpdateNeeded": false
  }]
}
```

### Reconnection

The webapp uses 5-second timeout per attempt, up to 5 retries. Messages are queued during disconnection and replayed on reconnect.

## State Subscriptions

### Register

```json
{"URI": "state/register", "messages": [{"state": "<category>"}]}
```

| Category | What you receive |
|----------|-----------------|
| `"module"` | Load level changes (`module.<addr>`, `rtc-out.<addr>`) |
| `"scene"` | Scene activation/deactivation (`scene.<id>`) |
| `"networkstats"` | Device online/offline/RSSI changes |
| `"all"` | Everything (used by the WebSocket Monitor at `/system/`) |

### Unregister

```json
{"URI": "state/unregister", "messages": [{"state": "<category>"}]}
```

### Receiving Updates

```json
{"URI": "state/update", "messages": [{"state": "<name>", "value": "<value>"}]}
```

#### Update types

| State prefix | Value format | Meaning |
|-------------|-------------|---------|
| `module.<hex-addr>` | `"50,100,X,-1"` | Load levels per offset. `X` = no change, `-1` = unknown |
| `scene.<id>` | `"1"` or `"0"` | Scene activated/deactivated |
| `switch.<hex-addr>` | `"press"`, `"hold"`, `"release"` | Button event |
| `load.<hex-addr>` | Level string | Load state change |
| `led.<hex-addr>` | `"1,0,1,0,0"` | LED indicator states |
| `ambient.<hex-addr>` | Numeric string | Ambient light sensor |
| `NetStatsUpdate` | `"<uid>,<ip>,<state>,<rssi>"` | Device network status |

#### Module update parsing

Comma-separated load levels, one per load on the device:
```
"50,100,X,-1"
  │    │   │  │
  │    │   │  └─ load 4: unknown
  │    │   └──── load 3: no change
  │    └──────── load 2: 100%
  └───────────── load 1: 50%
```

The webapp debounces: updates within 333ms of the last user command are ignored.

## State Write — `state/set`

The primary mechanism for controlling hardware:

```json
{"URI": "state/set", "messages": [{"state": "<name>", "value": "<value>"}]}
```

### Address Calculation

All hardware state names use a hex address:

```
hexAddress = (parseInt(deviceAddress, 16) << 16) | (offset & 0x3FF)
```

Example: device `"005"`, offset `0` → `(5 << 16) | 0` = `0x50000`

### Load Control

| State | Value | Effect |
|-------|-------|--------|
| `load.<addr>` | `"75%"` | Set dimmer to 75% (default ramp) |
| `load.<addr>` | `"75%.0"` | Set dimmer to 75% (immediate) |
| `load.<addr>` | `"0%.1"` | Toggle off |

### Button Press Simulation

Tap (press + 200ms + release):
```json
{"URI": "state/set", "messages": [{"state": "switch.50002", "value": "press"}]}
// wait 200ms
{"URI": "state/set", "messages": [{"state": "switch.50002", "value": "release"}]}
```

Button simulation is how we activate natural light mode — simulating the physical NL toggle button preserves the daylight curve. See `docs/appendix-a-natural-light.md`.

### Scene Activation

```json
{"URI": "state/set", "messages": [{"state": "scene.4", "value": "1"}]}   // activate
{"URI": "state/set", "messages": [{"state": "scene.4", "value": "0"}]}   // deactivate
```

### BLE Color Control

```json
{
  "URI": "state/set",
  "messages": [{
    "state": "lighting.lighting_controller.setcolor.<hexAddr>",
    "value": "<hexAddr>.<level>.<R>.<G>.<B>.<W>.<kelvin>.<colorType>"
  }]
}
```

Color types: `1` = RGB/RGBW, `3` = Kelvin only, `4` = dimming curve.

## State Queries

```json
{"URI": "state/module/<addr>/get", "messages": [{}]}       // load levels
{"URI": "state/modulecolors/<addr>/get", "messages": [{}]}  // color info
{"URI": "state/led/<addr>/get", "messages": [{}]}           // LED states
{"URI": "state/calibrationmode/<addr>/get", "messages": [{}]}
{"URI": "state/knob/<addr>/get", "messages": [{}]}
```

Note: generic `state/get` is **not supported** (returns "No route found").

## Config CRUD

Consistent URI pattern for all config types:

```json
{"URI": "lighting/config/<type>/get", "messages": [{}]}                // list
{"URI": "lighting/config/<type>/<id>/get", "messages": [{}]}          // get by ID
{"URI": "lighting/config/<type>/create", "messages": [{<object>}]}    // create
{"URI": "lighting/config/<type>/<id>/update", "messages": [{<object>}]}  // update
{"URI": "lighting/config/<type>/<id>/delete", "messages": [{"id": "<id>"}]} // delete
```

Types: `device`, `unbounddevice`, `networkstats`, `scene`, `rule`.

### Scene Object

```json
{
  "id": "1",
  "name": "Movie Night",
  "type": "default",
  "comparator": "0",
  "ramptime": "3",
  "load": [
    {"address": "005", "loadoffset": "1", "preset": "90", "min": "10", "max": "90",
     "fadeon": "3", "fadeoff": "3", "type": "D"}
  ]
}
```

Load types: `"D"` = dimmable, `"S"` = switch, `"F"` = fan, `"C"` = color.

## Rule/Timer Automation

Rules activate or deactivate scenes on a schedule, with optional celestial time references.

```json
{
  "URI": "lighting/config/rule/create",
  "messages": [{
    "name": "Porch Lights On at Sunset",
    "scene": "4",
    "function": "on",
    "time": "-00:15",
    "timeref": "sunset",
    "duration": "",
    "type": "daily",
    "days": "0,1,2,3,4,5,6",
    "active": "true"
  }]
}
```

| Field | Values |
|-------|--------|
| `function` | `"on"`, `"off"`, `"on/off"` (auto-off after duration), `"preset"` |
| `timeref` | `"time"` (fixed clock), `"sunrise"`, `"sunset"` |
| `time` | `"HH:MM"` or `"-HH:MM"` (negative = before reference) |
| `days` | `"0,1,2,3,4,5,6"` (0=Sunday) |

Note: `lighting/config/rule/<id>/get` is NOT supported — use the list endpoint.

## Health & Firmware

```json
{"URI": "lighting/health/system/stats/get", "messages": [{}]}
{"URI": "lighting/health/device/analysis/get", "messages": [{}]}
{"URI": "lighting/health/device/get", "messages": [{}]}
{"URI": "lighting/firmware/status", "messages": []}
```

## Verified Test Results

1. **Explicit brightness**: `load.50000` set to `87%` → physical dimmer changed to 87%
2. **NL toggle button**: `switch.50002` press+release → `NaturalLightIsOn=1`
3. **Scene toggle button**: `switch.50004` press+release → lights on at daylight curve level

## Source Files

The webapp JS files that implement this protocol are served from `http://<host>:80`:

| File | Purpose |
|------|---------|
| `WebSocketSrvc.js` | Core WebSocket transport |
| `LightCtrlSrvc.js` | High-level API wrapping all message types |
| `DataCacheService.min.js` | Startup data loading |
| `DeviceControlController.min.js` | Device control (buttons, loads, colors) |
| `LoadLevelService.min.js` | Load level management, color, light shows |
