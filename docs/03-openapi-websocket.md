# openapi-go Feedback WebSocket

Read-only WebSocket for subscribing to room-level state changes. Served by `openapi-go`.

## Connection

```
URL:         ws://192.168.4.96:3060/feedback/v1/register  (external, no auth)
             ws://127.0.0.1:3062/feedback/v1/register     (on-host)
Subprotocol: none
Auth:        none on port 3060
```

nginx proxies WebSocket upgrades on port 3060 to openapi-go on 3062.

## Protocol

All messages use JSON with a `URI` field and `messages` array.

### Subscribe

```json
{
  "URI": "feedback/state/register",
  "messages": [{"states": ["Den.BrightnessLevel", "Living.RoomLightsAreOn"]}]
}
```

Current values are pushed immediately on subscribe, then again on every change.

### Receive Updates

```json
{
  "uri": "feedback/state/update",
  "messages": [{"Den.BrightnessLevel": "95"}]
}
```

Note: outgoing uses `"URI"` (uppercase), incoming uses `"uri"` (lowercase).

### Unsubscribe

```json
{
  "URI": "feedback/state/unregister",
  "messages": [{"states": ["Den.BrightnessLevel"]}]
}
```

## Supported URIs

Only three URIs are supported. Everything else is silently ignored.

| Direction | URI |
|-----------|-----|
| → send | `feedback/state/register` |
| → send | `feedback/state/unregister` |
| ← recv | `feedback/state/update` |

## What Does NOT Work

Exhaustive testing confirmed that this WebSocket is **strictly read-only**:
- `state/set`, `feedback/state/set` — silently ignored
- `control/lighting/*/command` — silently ignored
- `servicerequest` — silently ignored
- `{"state": "..."}` (singular key instead of `"states"`) — silently ignored
- `{"URI": "state/register", ...}` (missing `feedback/` prefix) — silently ignored

## Behavior Notes

- The server sends WebSocket ping frames periodically — client must respond with pong (most WS libraries handle this automatically)
- State values are always strings (e.g., `"95"` not `95`)
- Multiple states can be registered in a single message
- State names use room-level format: `<RoomName>.<PropertyName>`

## Stability Caveat

The server may become unresponsive after prolonged connections (suspected memory leak in openapi-go). Mitigate by periodically disconnecting and reconnecting (e.g., every 5–10 minutes).

## When to Use This vs avc WebSocket

| | openapi-go WS | avc WS |
|---|---|---|
| **State names** | Room-level (`Den.BrightnessLevel`) | Hardware-level (`module.50000`) |
| **Auto-hydration** | Yes — current values pushed on subscribe | No |
| **Hardware control** | No | Yes |
| **Best for** | Monitoring dashboards, simple reads | Full integration with control |

For the MQTT bridge, we use the **avc WebSocket** instead (see `docs/04-avc-websocket.md`) because it supports both subscriptions and control in a single connection.
