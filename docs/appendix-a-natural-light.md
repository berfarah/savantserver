# Appendix A: Natural Light / Daylight Mode

## What It Does

Savant's "Natural Light" automatically adjusts dimmer levels based on time of day using a brightness/color-temperature curve managed by `dcm` (DynamicConfigManager). When a physical switch is pressed, loads turn on at the **natural light target level** (e.g., 92% in the evening) rather than 100%.

## How to Turn On with Natural Light

Simulate the room's NL toggle button via avc WebSocket (press + 200ms + release):

```json
{"URI": "state/set", "messages": [{"state": "switch.50004", "value": "press"}]}
// wait 200ms
{"URI": "state/set", "messages": [{"state": "switch.50004", "value": "release"}]}
```

### Verified Result (Den)

```
Before:  BrightnessLevel=0, RoomLightsAreOn=0, NaturalLightIsOn=0
Curve:   global.NaturalLightCurrentTemperatureBrightness = "4216K,92%"
After:   BrightnessLevel=92, RoomLightsAreOn=1, NaturalLightIsOn=1
```

## Toggle Mapping

At startup, find each room's NL toggle button from the REST API:

1. Fetch `GET /config/v1/lighting/devices/{id}` per device
2. Find buttons with `function: "naturallighttogglewithoff"` or `"naturallighttoggle"`
3. Compute switch address: `(parseInt(device.address, 16) << 16) | ((button.index - 1) & 0x3FF)`

Example: Den Hallway (address `005`), button index 3 → `(5 << 16) | 2` = `0x50002`

## Key Insight

Any scene toggle on a `followDaylight=true` load automatically applies the natural light curve. The button function type (`naturallighttogglewithoff` vs `toggle`) doesn't matter — the firmware applies the curve based on the load's `followDaylight` flag.

## What Does NOT Respect Natural Light

| Command | Result |
|---------|--------|
| `__RoomSetBrightness 100` via sclibridge | Sets to curve value but **disables tracking** |
| Scene activation via REST `/apply` | Ignores curve entirely |
| `DimmerSet` with explicit level | Overrides everything |
| REST `NaturalLightOn` command | Enables NL mode but doesn't turn lights on |

## Summary

| Method | Respects Curve? | Enables Tracking? | Turns On? |
|--------|----------------|-------------------|-----------|
| Physical button press | YES | YES | YES |
| **avc WS button simulation** | **YES** | **YES** | **YES** |
| REST scene `/apply` | NO | NO | YES |
| `__RoomSetBrightness` | Partially | NO | YES |

## Relevant State Names

- `global.NaturalLightCurrentTemperatureBrightness` — Current target: `"4216K,92%"`
- `<Room>.NaturalLightIsOn` — Whether NL mode is active for this room (0/1)
- `Farah SEA.RacePointMedia_host.NaturalLightIsEnabled_1_<addr>` — Per-load participation (0/1)

## Button Functions

| Function | Description |
|----------|-------------|
| `naturallighttogglewithoff` | Toggle with daylight curve |
| `naturallighttoggle` | Toggle with natural light (variant) |
| `toggle` | Simple scene toggle — also respects daylight on followDaylight loads |
| `on` / `off` | Simple on/off |
| `dynamicdimming` | Up/down dimming buttons |
