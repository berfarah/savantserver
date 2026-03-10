# sclibridge CLI Reference

Command-line interface for interacting with the Savant smart host. Used as a fallback when REST or WebSocket APIs are insufficient.

## Overview

- **Path**: `/usr/local/bin/sclibridge`
- **Type**: ARM 32-bit Objective-C binary (GNUstep)
- **IPC**: NNG over Unix domain sockets (state ops) and GNUstep DO (scene ops, service requests)
- **Dependency**: Requires `LD_LIBRARY_PATH=/usr/local/lib:$LD_LIBRARY_PATH`
- **Overhead**: ~50-100ms per invocation (process fork + library load + IPC connect)

## Commands

### State Operations (NNG IPC → rpmStateCenter)

```bash
sclibridge readstate <name>              # Read a state value
sclibridge writestate <name> <value>     # Write a state value
sclibridge statenames                     # List all state names (expensive!)
sclibridge writeStatesFile               # Persist state dictionary to disk
```

### Service Requests (GNUstep DO → SVC)

Takes **7 arguments** (not 5 as `sclibridge help` suggests):

```bash
sclibridge servicerequest <zone> <sourceComponent> <logicalComponent> <variantID> <serviceType> <command> <jsonArgs>
```

Examples:
```bash
# Switch on lights
sclibridge servicerequest Den 'Farah SEA' RacePointMedia_host 1 SVC_ENV_LIGHTING SwitchOn '{}'

# Set dimmer level
sclibridge servicerequest Den 'Farah SEA' RacePointMedia_host 1 SVC_ENV_LIGHTING DimmerSet '{"Address1":"005","DimmerLevel":"75"}'

# Set room brightness
sclibridge servicerequest Den 'Farah SEA' RacePointMedia_host 1 SVC_ENV_LIGHTING __RoomSetBrightness '{"BrightnessLevel":"50"}'
```

Dash-separated shorthand:
```bash
sclibridge servicerequestcommand 'Den-Farah SEA-RacePointMedia_host-1-SVC_ENV_LIGHTING-SwitchOn'
```

### Scene Operations (GNUstep DO → scli-scene-request.ipc)

```bash
sclibridge getSceneNames                          # List scenes
sclibridge activateScene <name> <id> <user>       # Activate a scene
sclibridge removeScene <name> <id> <user>         # Remove a scene
```

### Zone Discovery (SQLite — no IPC)

```bash
sclibridge userzones                    # List user zones
sclibridge servicesforzone <zone>       # List services for a zone
```

These read directly from `serviceImplementation.sqlite` with zero IPC connections.

### Triggers (GNUstep DO → SVC — currently non-functional)

```bash
sclibridge settrigger <name> <transitionCount> <matchType> <scope> <stateName> <logic> <data> <prematchCount> [prematch...] <zone> <sourceComp> <logicalName> <variantID> <serviceType> <request> <argName> <argValue> [...]

sclibridge removetrigger <name>
```

Blocked on this host due to missing `staticCommEnabled.plist`.

## When to Use sclibridge vs Other APIs

| Operation | Best Path | Why |
|-----------|-----------|-----|
| State reads | REST API | No process fork, JSON response |
| State writes | NNG IPC (go-mangos) | ~1ms vs ~100ms |
| Hardware control | avc WebSocket | Direct hardware access |
| State subscriptions | avc WebSocket | Push-based, real-time |
| State name enumeration | sclibridge `statenames` | No REST equivalent |
| Reactive triggers | sclibridge `settrigger` | No REST equivalent |
| Scene activate/list | REST API or avc WS | Avoid process fork |

The MQTT bridge does not use sclibridge — it uses REST for discovery and avc WebSocket for control.
