# Appendix B: NNG IPC Protocol

Low-level details of the NNG IPC layer used by Savant's internal processes.

## Wire Protocol (from strace)

### Connection Sequence

1. Connect to Unix domain socket (e.g., `/tmp/stateCenter-request.ipc`)
2. Send SP handshake (8 bytes): `\x00\x53\x50\x00\x00\x30\x00\x00`
3. Send request as 3-part `sendmsg` iov
4. Receive JSON response

### SP Handshake

```
\x00SP\x00\x00\x30\x00\x00   (8 bytes)
  │  ││    │    │
  │  │└────┘    └── Padding/version
  │  └── "SP" = Scalability Protocols identifier
  └── NUL prefix

SP type 0x0030 = REQ (request) socket
```

Sent once per connection, not per message.

### Request Message

```
iov[0]: Header     (9 bytes)  — fixed prefix + payload length byte
iov[1]: Request ID (4 bytes)  — correlates response
iov[2]: JSON payload (variable)
```

### JSON Operations

All go through `/tmp/stateCenter-request.ipc`:

```json
{"URI":"state/query","request":{"states":["Den.BrightnessLevel"]}}
{"URI":"state/update","request":{"Den.BrightnessLevel":"50"}}
{"URI":"state/names","request":{}}
{"URI":"state/writeStateDictionary","request":{}}
```

Note: the NNG URI is `state/update`, not `state/set` (the avc WebSocket uses `state/set` but translates internally).

## Go Implementation

The `go-mangos` library handles SP handshake and framing automatically:

```go
import (
    "go.nanomsg.org/mangos/v3/protocol/req"
    _ "go.nanomsg.org/mangos/v3/transport/ipc"
)

sock, _ := req.NewSocket()
sock.Dial("ipc:///tmp/stateCenter-request.ipc")
sock.Send([]byte(`{"URI":"state/query","request":{"states":["Den.BrightnessLevel"]}}`))
msg, _ := sock.Recv()
```

## Key IPC Sockets

| Socket | Protocol | Purpose |
|--------|----------|---------|
| `stateCenter-request.ipc` | NNG/JSON | State read/write (sclibridge, avc) |
| `stateCenter-openAPI-request.ipc` | NNG/JSON | Dedicated openapi-go state channel |
| `stateCenter-post.ipc` | NNG/JSON | State change notifications |
| `openapi-ltc-request.ipc` | NNG/JSON | Lighting config CRUD |
| `openapi-svc-request.ipc` | NNG/JSON | Services, HVAC, shades, scenes |
| `dcm-request.ipc` | NNG/JSON | Rooms, configuration |
| `scli-scene-request.ipc` | GNUstep DO | Scene operations (binary, not JSON) |
| `NSMessagePort/ports/939.10` | GNUstep DO | Service requests to SVC |

84 total IPC sockets exist on the system. See the full inventory in the source docs.

## Limitations

- **No push subscriptions**: `stateCenter-post.ipc` is REQ/REP, returns empty payloads to external clients
- **No hardware control**: NNG only carries state values; avc translates WS commands to hardware
- **Scene ops require ObjC runtime**: GNUstep DO, not replicable from Go
