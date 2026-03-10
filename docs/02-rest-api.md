# openapi-go REST API

The REST API is served by `openapi-go`, a Go binary on the Savant host. It provides read-only access to configuration data and state values. It does **not** support hardware control.

## Connection Details

| | Internal (on host) | External (from network) |
|---|---|---|
| **Port** | 3062 (HTTP), 3063 (HTTPS) | 3060 (HTTP, no auth), 3061 (HTTPS + auth) |
| **Auth** | None | None on 3060, required on 3061 |
| **Notes** | Bypasses nginx | Via nginx reverse proxy |

For `savantserver` running on the host, use port 3062 directly.

## Configuration Endpoints

| Method | Endpoint | Returns |
|--------|----------|---------|
| GET | `/config/v1/rooms` | All rooms with UUIDs |
| GET | `/config/v1/rooms/{RoomID}` | Single room |
| GET | `/config/v1/room/{RoomID}/loads` | Loads for a specific room |
| GET | `/config/v1/lighting/devices` | All lighting devices (41) |
| GET | `/config/v1/lighting/devices/{id}` | Device with loads and buttons |
| GET | `/config/v1/lighting/loads` | All lighting loads (40) |
| GET | `/config/v1/lighting/loads/{id}` | Single load |
| GET | `/config/v1/lighting/scenes` | All lighting scenes (55) |
| GET | `/config/v1/lighting/scenes/{id}` | Single scene with members |
| GET | `/config/v1/lighting/groups` | Lighting groups |
| GET | `/config/v1/lighting/buttons/{id}` | Button config |
| GET | `/config/v1/configuration/active` | Active config info |
| GET | `/config/v1/location/active` | Location with lat/lon, timezone |
| GET | `/config/v1/services` | All services (39) |
| GET | `/config/v1/services/{id}/commands` | Commands for a service |
| GET | `/config/v1/services/{id}/commands/{cmd}/arguments` | Arguments for a command |
| GET | `/config/v2/services/events` | All service commands by room |
| GET | `/config/v1/hvac/components` | HVAC components |
| GET | `/config/v1/shades/groups` | Shade groups |

## State Endpoints

| Method | Endpoint | Returns |
|--------|----------|---------|
| GET | `/states/{stateName}` | `{"data":["<value>"]}` |
| GET | `/states/registered` | List of registered state subscriptions |

State name format: `<RoomName>.<PropertyName>` (e.g., `Den.BrightnessLevel`).

Examples:
```
GET /states/Den.BrightnessLevel        → {"data":["95"]}
GET /states/Living.RoomLightsAreOn     → {"data":["0"]}
GET /states/Den.NumberOfLightsOn       → {"data":["1"]}
```

## System Endpoints

| Method | Endpoint | Returns |
|--------|----------|---------|
| GET | `/system/v1/ping` | `{"version":"1.6","schemaVersion":"1.2"}` |
| GET | `/system/v1/network` | Network interfaces |
| GET | `/system/v1/processes` | Running processes with stats |
| GET | `/system/v1/processes/{id}` | Single process |
| GET | `/system/v1/cpu` | CPU usage |

## Control Endpoints (Non-Functional)

**All POST control endpoints return `{"result":"success"}` but do NOT change hardware state.** PUT variants return HTTP 402 (auth-gated).

| Method | Endpoint | Status |
|--------|----------|--------|
| POST | `/control/v1/room/{RoomID}/lighting/command` | Returns success, no effect |
| POST | `/control/v1/lighting/loads/{LoadID}/level` | Returns success, no effect |
| POST | `/control/v1/lighting/scenes/{id}/apply` | Returns success, no effect |
| PUT | `/control/lighting/...` | HTTP 402 |

For hardware control, use the avc WebSocket (port 8480) — see `docs/04-avc-websocket.md`.

## Data Models

### Room
```json
{
  "roomID": "2AE47528-2963-45EB-A3B5-7E2ECF438891",
  "configurationID": "DCCB28B8-B4DC-4A06-B202-AC65DD3972A0",
  "name": "Den",
  "type": "user",
  "followDaylight": null
}
```

### Load
```json
{
  "loadID": "0134E03E-AB83-49D8-A7A6-C4DB54DE9EAD",
  "deviceID": "642DA5F1-EB43-4F43-957D-1858599AC144",
  "roomID": "2AE47528-2963-45EB-A3B5-7E2ECF438891",
  "name": "Lights",
  "offset": 0,
  "type": 0,
  "isWired": true,
  "min": 0,
  "max": 100,
  "tracksDevice": true,
  "followDaylight": true
}
```
- `type`: 0 = dimmable, 1 = switch
- `min`/`max`: dimmer range (some loads have min=10 or min=15)
- `followDaylight`: whether load participates in natural light daylight curve

### Device (with loads and buttons)
```json
{
  "deviceID": "642DA5F1-EB43-4F43-957D-1858599AC144",
  "roomID": "2AE47528-2963-45EB-A3B5-7E2ECF438891",
  "address": "005",
  "name": "Den Hallway",
  "type": "ECHVAP4D",
  "boardName": "4 Button with Up/Down Keypad",
  "hasWiredLoad": true,
  "hasEnergyMonitoring": true,
  "ambient": true,
  "isConnected": false,
  "RPMLightingDeviceName": "ECHO Adaptive phase",
  "loads": [/* Load objects */],
  "buttons": [/* Button objects */]
}
```

Note: the list endpoint (`/config/v1/lighting/devices`) does **not** include `loads` or `buttons` arrays. Fetch per-device detail to get those.

### Button
```json
{
  "buttonID": "DD4DC039-A76E-4489-93AF-C09CA1518B0A",
  "deviceID": "642DA5F1-EB43-4F43-957D-1858599AC144",
  "lightingSceneID": "",
  "RoomID": "2AE47528-2963-45EB-A3B5-7E2ECF438891",
  "label": "Den",
  "scenes": "",
  "index": 3,
  "function": "naturallighttogglewithoff",
  "command": "NaturalLightOn",
  "toggleCommand": "NaturalLightOffWithCommand",
  "ledBehavior": 0
}
```

Note: `RoomID` has uppercase `R` in JSON (inconsistent with other models).

### Scene
```json
{
  "lightingSceneID": "8F1A30E9-2A8D-4BD3-8B37-661C75E20813",
  "name": "Den Lights Scene",
  "type": 4,
  "sceneIndex": 19,
  "rampTime": 2,
  "dimmerPreset": 100,
  "comparator": 4,
  "members": [{"loadID": "0134E03E-...", "name": "Lights", "type": 0}]
}
```

## Advantages Over sclibridge

- No process fork per request
- Standard HTTP — works from any language
- JSON responses (no string parsing)
- Structured config data with UUIDs and relationships
