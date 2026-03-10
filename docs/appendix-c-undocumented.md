# Appendix C: Undocumented Systems & Unexplored Leads

Items discovered during reverse engineering that aren't used by the MQTT bridge but may be valuable for future work.

## High Value

### Door Lock Controllers

Three door lock controllers are configured:
- `com.savant.avc-Door Lock Controller.plist`
- `com.savant.avc-Front Door Lock Controller.plist`
- `com.savant.avc-Back Door Lock Controller.plist`

Likely controllable via `servicerequest` but service IDs and commands haven't been enumerated.

### HomeKit Bridge (`hkb`)

1.2MB native binary at `/usr/local/bin/hkb`. Not currently running. If functional, this could provide HomeKit integration directly.

### NNG Pub/Sub Transports

The sclibridge binary contains strings suggesting additional NNG capabilities beyond REQ/REP:
- `nanoIPCTypePubSubClient` / `nanoIPCTypePubSubServer`
- `nanoTransportTypeWS` / `nanoTransportTypeWSS` (NNG over WebSocket)
- `pubsub` binary exists at `/usr/local/bin/pubsub`

### Unidentified Ports

| Port | Notes |
|------|-------|
| 3050 | Adjacent to REST API ports — could be another openapi-go endpoint |
| 3051 | Same as above |
| 9090 | Common for web UIs / metrics |

## Medium Value

### Additional Processes

| Process | Purpose |
|---------|---------|
| avcEmb | Embedded AV controller, UDP 48656/48658 |
| security-daemon | Auth/security |
| savhost-metrics-collector | Host metrics |
| timecop | Clock change detection |
| DynamicConfigManager (dcm) | Natural light curves, config |

### Active Network Connections

- **edm → 192.168.6.201:40000** — external device on different subnet
- **Debug and Management Server → 54.68.155.173:443** — AWS (Savant cloud telemetry)

### mDNS Services

- `_smor._tcp` — Savant proprietary device discovery
- `_companion-link._tcp` — Apple AirPlay 2

### Notable Systemd Services

- `atftpd.service` — TFTP for firmware updates to lighting hardware
- `pulseaudio.service` — System-wide audio with Savant config

## Low Value

Additional binaries not running: `simpleCLI`, `schttp` ("State Center HTTP"), `scweb`, `nanoTest`, `AuthServer`, `CameraServer`, `SimpleServer`, and others.

Hardware: Freescale i.MX6 Quad/DualLite, 4 cores, kernel 4.14.78 SMP PREEMPT.
