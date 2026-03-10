# Savant System Architecture

## Hardware

- **Model**: SHR-2000-00 (Savant Smart Host)
- **SoC**: Freescale i.MX6 Quad/DualLite, ARMv7l (32-bit ARM), 4 CPU cores
- **OS**: Savant Embedded Linux 20.04 (Ubuntu-based), kernel 4.14.78
- **Hostname**: `sav-001aae11f0260000`
- **Configuration**: "Farah SEA" (originally "RacePoint Media" — RPM)
- **IP**: 192.168.4.96
- **SSH**: `RPM@192.168.4.96` (key-based, no password)

## Software Stack

All Savant binaries are ARM 32-bit ELF Objective-C on GNUstep, **not stripped, with debug_info**. The system was originally built by "RacePoint Media" (RPM), later rebranded to Savant.

## Key Processes

| Process | Binary | Role | Port(s) |
|---------|--------|------|---------|
| rpmStateCenter | `/usr/local/bin/rpmStateCenter` | Central state management hub | TCP 11280, UDP 11251/11252 |
| SVC | `/usr/local/bin/SVC` | Service controller — executes commands | UDP 11250 |
| avc | `/usr/local/bin/avc` | Lighting controller, hosts web app WS | TCP 8480 |
| openapi-go | `/usr/bin/openapi-go` | REST + WebSocket API (Go binary) | TCP 3062 (HTTP), 3063 (HTTPS) |
| nginx | system | Reverse proxy for openapi-go | TCP 80, 443, 3060, 3061 |
| Debug and Management Server | `/usr/local/bin/Debug and Management Server` | Management interface | TCP 9108, 9119 |
| edm | `/usr/bin/edm` | External device manager | TCP 3401, 9001, UPnP 1900 |
| rpmMonitord | `/usr/local/bin/rpmMonitord` | System monitor | TCP 1124, UDP 11259 |
| DIS (×5) | `/usr/local/bin/DIS` | Device interface server | various UDP |
| dcm | `/usr/local/bin/dcm` | Dynamic config manager | various UDP |
| startupManager | `/usr/local/bin/startupManager` | Startup/shutdown orchestration | UDP 12001-12007 |

## Communication Topology

```
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   Savant App     │     │  savantserver     │     │  Lighting Web    │
│   (iOS/Mac)      │     │  (this project)   │     │  App (:80/:443)  │
└────────┬─────────┘     └────────┬─────────┘     └───┬──────────┬───┘
         │                        │                    │          │
         │ (proprietary)          │ REST/WS            │ static   │ WS
         │                        │ port 3062          │ files    │ port 8480
         ▼                        ▼                    ▼          │
┌──────────────────────────────────────────────────────────┐      │
│                    nginx (reverse proxy)                  │      │
│  :80/:443 → static /www/  (lighting web app HTML/JS/CSS) │      │
│  :3060 → :3062 (openapi-go, no auth)                     │      │
│  :3061 → :3063 (openapi-go, SSL + auth)                  │      │
└────────┬────────────────────────┬────────────────────────┘      │
         │                        │                               │
         ▼                        ▼                               ▼
┌──────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│   openapi-go     │     │   sclibridge     │     │       avc        │
│   :3062 / :3063  │     │   (CLI tool)     │     │      :8480       │
│   REST + WS      │     │                  │     │  lighting WS     │
└────────┬─────────┘     └────────┬─────────┘     └────────┬─────────┘
         │                        │                        │
         │ NNG IPC                │ NNG IPC                │ (internal)
         ▼                        ▼                        ▼
┌──────────────────────────────────────────────────────────────────────┐
│                    NNG IPC Layer (Unix Domain Sockets)                │
│   /tmp/stateCenter-request.ipc   /tmp/stateCenter-post.ipc          │
│   /tmp/svc-request.ipc           /tmp/scli-scene-request.ipc        │
│   84 total .ipc sockets                                              │
└────────┬─────────────────────────┬───────────────────────────────────┘
         │                        │
         ▼                        ▼
┌──────────────────┐     ┌──────────────────┐
│  rpmStateCenter  │     │       SVC        │
│  (state hub)     │     │  (service ctrl)  │
└──────────────────┘     └──────────────────┘
```

### Port Routing

- **Port 80/443**: nginx serves static files from `/www/` (the lighting web app). Not proxied.
- **Port 3060/3061**: nginx proxies to openapi-go (:3062/:3063). Includes WebSocket upgrade for `/feedback/v1/register`.
- **Port 8480**: avc listens directly — **not proxied through nginx**. The webapp's JavaScript connects here for all lighting control.

## IPC Mechanisms

Three distinct IPC mechanisms exist on the system:

| Mechanism | Format | Used For | Replicable from Go? |
|-----------|--------|----------|---------------------|
| **NNG (SP protocol)** | JSON payloads with binary framing | State read/write/list | Yes — via `go-mangos` |
| **GNUstep DO (IPC)** | Binary Objective-C RPC over `/tmp/*.ipc` | Scene operations, avc internal | No — requires ObjC runtime |
| **GNUstep DO (NSMessagePort)** | Binary Objective-C RPC via `/tmp/GNUstepSecure1000/NSMessagePort/ports/<PID>.10` | Service requests to SVC | No — requires ObjC runtime |

The original `REVERSE_ENGINEERING.md` hypothesized GNUstep Distributed Objects (DO) for all IPC. This was **disproven** — while binaries are GNUstep/Objective-C, state operations use **NNG (nanomsg next generation)** over Unix domain sockets. `gdomap` is not running. DO is only used for scene operations and service requests.

## Physical Infrastructure

- 20 rooms (18 standard + Shared Equipment, Unassigned)
- 41 lighting devices (switches/keypads)
- 40 lighting loads
- 55 lighting scenes
- 38 unique load addresses (3-digit hex: 001–02E, with gaps)

## Key Libraries

| Library | Purpose |
|---------|---------|
| `libsavantNNG.so` | Core NNG IPC transport |
| `librpmStateManagement.so` | State management |
| `librpmCommonIPC.so` | Common IPC patterns |
| `librpmiLumi.so` / `libiLumi.so` | iLumi lighting integration |
| `librpmKNX.so` | KNX protocol |
| `librpmBACNet.so` | BACNet HVAC |

## External APIs

The system exposes three external APIs, each documented separately:

- **openapi-go REST API** (port 3062) — read-only config discovery and state queries. See `docs/02-rest-api.md`.
- **openapi-go WebSocket** (port 3060) — read-only state subscriptions. See `docs/03-openapi-websocket.md`.
- **avc WebSocket** (port 8480) — hardware control and state subscriptions. See `docs/04-avc-websocket.md`.
- **sclibridge CLI** — command-line interface to IPC layer. See `docs/05-sclibridge.md`.
