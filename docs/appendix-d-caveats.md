# Appendix D: Caveats & Known Issues

## 1. WebSocket Connection Stability

The openapi-go WebSocket may become unresponsive after prolonged connections (suspected memory leak). Mitigate by periodically disconnecting and reconnecting (the bridge does this every 5 minutes for the avc WS).

## 2. REST Control Endpoints Don't Work

All `POST /control/v1/...` endpoints return `{"result":"success"}` but do **not** change hardware state. `PUT /control/lighting/...` returns HTTP 402. Use the avc WebSocket for all hardware control.

## 3. Port 3060 Has No Authentication

Anyone on the local network can read state and config via port 3060. Ensure network segmentation. The bridge uses port 3062 (localhost only) to avoid this.

## 4. State Values Are Always Strings

All state values are returned as strings: `"95"` not `95`, `"0"` or `"1"` for booleans, `"4359K,94%"` for composite values.

## 5. Room Names Have Special Characters

Room names contain spaces and apostrophes (`Evelyn's Room`, `Kids' Bath`). These need URL encoding in REST calls and slug conversion for MQTT topics. The bridge handles this via `Slugify()`.

## 6. `statenames` Is Expensive

From sclibridge help: "running this command can impact system performance. It should be run as infrequently as possible." Cache results if needed.

## 7. sclibridge servicerequest Takes 7 Arguments

The `sclibridge help` documentation is misleading — it shows 5 arguments but the command takes 7: `zone`, `sourceComponent`, `logicalComponent`, `variantID`, `serviceType`, `command`, `jsonArgs`.

## 8. Natural Light Bypass

Using `DimmerSet` with an explicit level, or activating a scene via REST, bypasses the natural light daylight curve. Only button press simulation (avc WS) preserves it. See `docs/appendix-a-natural-light.md`.

## 9. Button JSON Field Inconsistency

The `RoomID` field in Button objects uses uppercase `R`, while all other models use lowercase `roomID`. The Go struct tag must account for this: `json:"RoomID"`.
