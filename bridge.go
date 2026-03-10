package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Bridge orchestrates the Savant-to-MQTT integration.
// It discovers entities from the Savant REST API, subscribes to hardware state
// via the avc WebSocket, and publishes/receives commands via MQTT for Home
// Assistant. See docs/06-design.md for the full lifecycle.
type Bridge struct {
	cfg    *Config
	api    *SavantAPI
	avc    *AVCClient
	avcMu  sync.Mutex   // protects avc client during reconnect
	mqtt   *MQTTClient
	logger *log.Logger

	entities  []LightEntity
	entityIdx map[string]*LightEntity // keyed by "room_slug/load_slug" for command lookup
	addrIdx   map[int]*LightEntity    // keyed by HexAddr for avc state update lookup

	state map[string]*LightState // keyed by UniqueID
	mu    sync.RWMutex           // protects state map
}

// NewBridge creates a new bridge with the given configuration.
func NewBridge(cfg *Config) *Bridge {
	return &Bridge{
		cfg:    cfg,
		logger: log.New(os.Stderr, "[bridge] ", log.LstdFlags),
	}
}

// Start runs the bridge lifecycle. Blocks until ctx is cancelled.
func (b *Bridge) Start(ctx context.Context) error {
	b.logger.Printf("starting (savant=%s:%d, avc=%d, mqtt=%s)",
		b.cfg.Savant.Host, b.cfg.Savant.RESTPort, b.cfg.Savant.AVCPort, b.cfg.MQTT.Broker)

	// 1. Discovery — fetch config from REST API
	if err := b.discover(ctx); err != nil {
		return fmt.Errorf("discovery: %w", err)
	}

	// 2. Hydrate state from REST API
	if err := b.hydrateState(ctx); err != nil {
		// Non-fatal: we can still start with empty state and get updates from avc WS
		b.logger.Printf("warning: state hydration failed: %v", err)
	}

	// 3. Connect MQTT
	b.mqtt = NewMQTTClient(b.cfg.MQTT, b.cfg.HA.DiscoveryPrefix)
	if err := b.mqtt.Connect(ctx); err != nil {
		return fmt.Errorf("mqtt connect: %w", err)
	}

	// 4. Publish discovery + initial state
	if err := b.mqtt.PublishDiscovery(b.entities); err != nil {
		return fmt.Errorf("mqtt publish discovery: %w", err)
	}
	if err := b.mqtt.PublishAvailability(true); err != nil {
		return fmt.Errorf("mqtt publish availability: %w", err)
	}
	b.publishAllStates()

	// 5. Subscribe to MQTT commands
	if err := b.mqtt.SubscribeCommands(b.handleCommand); err != nil {
		return fmt.Errorf("mqtt subscribe commands: %w", err)
	}

	// 6. Connect avc WebSocket
	if err := b.connectAVC(ctx); err != nil {
		return fmt.Errorf("avc connect: %w", err)
	}

	// 7. Start background loops
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		b.avc.ReadLoop(ctx, b.handleStateUpdate)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		b.reconnectLoop(ctx)
	}()

	// Block until shutdown
	<-ctx.Done()
	b.logger.Println("shutting down")

	b.mqtt.Close()
	b.avcMu.Lock()
	if b.avc != nil {
		b.avc.Close()
	}
	b.avcMu.Unlock()
	wg.Wait()

	b.logger.Println("stopped")
	return nil
}

// discover fetches rooms, loads, and devices from the REST API and builds
// the entity registry with lookup indices.
func (b *Bridge) discover(ctx context.Context) error {
	b.api = NewSavantAPI(b.cfg.Savant.Host, b.cfg.Savant.RESTPort)

	b.logger.Println("fetching rooms, loads, devices...")

	rooms, err := b.api.FetchRooms(ctx)
	if err != nil {
		return err
	}

	loads, err := b.api.FetchLoads(ctx)
	if err != nil {
		return err
	}

	devices, err := b.api.FetchDevices(ctx)
	if err != nil {
		return err
	}

	// Fetch per-device detail (with buttons) for natural light toggle mapping
	b.logger.Printf("fetching button configs for %d devices...", len(devices))
	detailedDevices := make([]Device, 0, len(devices))
	for _, d := range devices {
		detailed, err := b.api.FetchDevice(ctx, d.DeviceID)
		if err != nil {
			b.logger.Printf("warning: could not fetch device %s detail: %v", d.DeviceID, err)
			detailedDevices = append(detailedDevices, d) // use list version without buttons
			continue
		}
		detailedDevices = append(detailedDevices, *detailed)
	}

	entities, err := BuildEntities(rooms, loads, detailedDevices)
	if err != nil {
		return fmt.Errorf("building entities: %w", err)
	}

	b.entities = entities
	b.state = make(map[string]*LightState, len(entities))
	b.entityIdx = make(map[string]*LightEntity, len(entities))
	b.addrIdx = make(map[int]*LightEntity, len(entities))

	for i := range b.entities {
		e := &b.entities[i]
		b.state[e.UniqueID] = &LightState{}
		b.entityIdx[e.RoomSlug+"/"+e.LoadSlug] = e
		b.addrIdx[e.HexAddr] = e
	}

	b.logger.Printf("discovered %d rooms, %d loads → %d entities",
		len(rooms), len(loads), len(entities))
	return nil
}

// hydrateState fetches initial brightness values from the REST API.
// When savant.config_name is set, fetches per-load dimmer levels for accurate
// state. Otherwise falls back to room-level brightness (shared across all loads
// in the room).
func (b *Bridge) hydrateState(ctx context.Context) error {
	configName := b.cfg.Savant.ConfigName

	if configName != "" {
		return b.hydratePerLoad(ctx, configName)
	}
	return b.hydratePerRoom(ctx)
}

// hydratePerLoad fetches per-load dimmer levels using the state name format:
// <configName>.RacePointMedia_host.CurrentDimmerLevel_1_<deviceAddr>
func (b *Bridge) hydratePerLoad(ctx context.Context, configName string) error {
	// Build state names for each entity's device address
	type loadQuery struct {
		stateName string
		entity    *LightEntity
	}
	var queries []loadQuery
	var stateNames []string
	seen := make(map[string]bool)

	for i := range b.entities {
		e := &b.entities[i]
		stateName := fmt.Sprintf("%s.RacePointMedia_host.CurrentDimmerLevel_1_%s",
			configName, e.DeviceAddress)
		if !seen[stateName] {
			seen[stateName] = true
			stateNames = append(stateNames, stateName)
		}
		queries = append(queries, loadQuery{stateName: stateName, entity: e})
	}

	b.logger.Printf("hydrating state for %d loads (%d unique devices)...", len(queries), len(stateNames))
	states, err := b.api.FetchStates(ctx, stateNames)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	hydrated := 0
	for _, q := range queries {
		val, ok := states[q.stateName]
		if !ok {
			continue
		}
		// The state value is the dimmer level for the first load on the device.
		// For multi-load devices, the value is comma-separated per offset.
		levels := strings.Split(val, ",")
		offset := q.entity.LoadOffset
		if offset >= len(levels) {
			continue
		}
		levelStr := levels[offset]
		if levelStr == "X" || levelStr == "-1" || levelStr == "" {
			continue
		}
		level, err := strconv.Atoi(levelStr)
		if err != nil {
			continue
		}
		st := b.state[q.entity.UniqueID]
		st.Brightness = level
		st.On = level > 0
		hydrated++
	}

	b.logger.Printf("hydrated %d/%d loads from per-load state", hydrated, len(queries))
	return nil
}

// hydratePerRoom fetches room-level brightness. All loads in a room share the
// same brightness value. This is a fallback when config_name is not set.
func (b *Bridge) hydratePerRoom(ctx context.Context) error {
	roomsSeen := make(map[string]bool)
	var stateNames []string
	for _, e := range b.entities {
		if roomsSeen[e.RoomName] {
			continue
		}
		roomsSeen[e.RoomName] = true
		stateNames = append(stateNames, e.RoomName+".BrightnessLevel")
		stateNames = append(stateNames, e.RoomName+".RoomLightsAreOn")
	}

	b.logger.Printf("hydrating state for %d rooms (per-room fallback)...", len(roomsSeen))
	states, err := b.api.FetchStates(ctx, stateNames)
	if err != nil {
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, e := range b.entities {
		st := b.state[e.UniqueID]

		if val, ok := states[e.RoomName+".RoomLightsAreOn"]; ok {
			st.On = val == "1"
		}
		if val, ok := states[e.RoomName+".BrightnessLevel"]; ok {
			if brightness, err := strconv.Atoi(val); err == nil {
				st.Brightness = brightness
				if brightness > 0 {
					st.On = true
				}
			}
		}
	}

	b.logger.Printf("hydrated state from %d values", len(states))
	return nil
}

// connectAVC connects to the avc WebSocket and subscribes to state updates.
func (b *Bridge) connectAVC(ctx context.Context) error {
	client := NewAVCClient(b.cfg.Savant.Host, b.cfg.Savant.AVCPort)

	if err := client.Connect(ctx); err != nil {
		return err
	}

	if err := client.Subscribe("module", "scene"); err != nil {
		client.Close()
		return fmt.Errorf("avc subscribe: %w", err)
	}

	b.avcMu.Lock()
	b.avc = client
	b.avcMu.Unlock()

	return nil
}

// reconnectLoop periodically reconnects the avc WebSocket to mitigate
// potential memory leaks in the Savant firmware.
func (b *Bridge) reconnectLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			b.logger.Println("periodic avc reconnect")

			b.avcMu.Lock()
			oldAVC := b.avc
			b.avcMu.Unlock()

			// Close old connection — causes old ReadLoop to exit
			if oldAVC != nil {
				oldAVC.Close()
			}

			if err := b.connectAVC(ctx); err != nil {
				b.logger.Printf("reconnect failed: %v (will retry in 5m)", err)
				continue
			}

			// Restart read loop with new client
			b.avcMu.Lock()
			newAVC := b.avc
			b.avcMu.Unlock()
			go newAVC.ReadLoop(ctx, b.handleStateUpdate)
		}
	}
}

// handleStateUpdate processes a state update from the avc WebSocket and
// publishes the new state to MQTT.
func (b *Bridge) handleStateUpdate(update AVCStateUpdate) {
	if strings.HasPrefix(update.State, "module.") {
		b.handleModuleUpdate(update)
	}
	// scene updates could be handled here in the future
}

// handleModuleUpdate processes a module state update.
// Module updates arrive as "module.<device_hex_addr>" with comma-separated
// load levels (one per load on the device).
func (b *Bridge) handleModuleUpdate(update AVCStateUpdate) {
	// Parse device address from "module.<hex>"
	addrStr := strings.TrimPrefix(update.State, "module.")
	deviceAddr, err := strconv.ParseInt(addrStr, 16, 64)
	if err != nil {
		return
	}

	// Split levels by comma — one per load offset
	levels := strings.Split(update.Value, ",")

	for offset, levelStr := range levels {
		if levelStr == "X" || levelStr == "-1" {
			continue // no change or unknown
		}

		level, err := strconv.Atoi(levelStr)
		if err != nil {
			continue
		}

		hexAddr := ComputeHexAddr(int(deviceAddr), offset)
		entity, ok := b.addrIdx[hexAddr]
		if !ok {
			continue // not a tracked load
		}

		b.mu.Lock()
		st := b.state[entity.UniqueID]
		st.On = level > 0
		st.Brightness = level
		b.mu.Unlock()

		if b.mqtt != nil {
			if err := b.mqtt.PublishState(entity, *st); err != nil {
				b.logger.Printf("publish state error for %s: %v", entity.UniqueID, err)
			}
		}
	}
}

// handleCommand processes a command received from HA via MQTT.
func (b *Bridge) handleCommand(entityID string, cmd LightCommand) {
	entity, ok := b.entityIdx[entityID]
	if !ok {
		b.logger.Printf("command for unknown entity: %s", entityID)
		return
	}

	b.logger.Printf("command for %s (%s %s): %+v", entityID, entity.RoomName, entity.Name, cmd)

	// Determine action and perform optimistic state update
	b.mu.Lock()
	st := b.state[entity.UniqueID]

	if cmd.State != nil {
		switch *cmd.State {
		case "ON":
			st.On = true
			if cmd.Brightness != nil {
				st.Brightness = *cmd.Brightness
			} else if st.Brightness == 0 {
				if entity.FollowDaylight {
					// NL loads will be set to daylight curve level — we don't
					// know the exact value, so leave brightness unset and let
					// the avc WS update correct it. Use -1 as a sentinel to
					// signal "on but brightness unknown" to the state publisher.
					st.Brightness = -1
				} else {
					st.Brightness = entity.Max
				}
			}
		case "OFF":
			st.On = false
			st.Brightness = 0
		}
	} else if cmd.Brightness != nil {
		st.Brightness = *cmd.Brightness
		st.On = *cmd.Brightness > 0
	}

	optimistic := *st
	b.mu.Unlock()

	// Publish optimistic state to MQTT (instant UI feedback)
	if b.mqtt != nil {
		if err := b.mqtt.PublishState(entity, optimistic); err != nil {
			b.logger.Printf("optimistic publish error for %s: %v", entity.UniqueID, err)
		}
	}

	// Send command to avc WebSocket
	b.avcMu.Lock()
	avc := b.avc
	b.avcMu.Unlock()
	if avc != nil {
		b.sendAVCCommand(avc, entity, cmd)
	}
}

// sendAVCCommand dispatches the appropriate avc WebSocket command for a light command.
func (b *Bridge) sendAVCCommand(avc *AVCClient, entity *LightEntity, cmd LightCommand) {
	if cmd.State != nil && *cmd.State == "OFF" {
		// Turn off
		if err := avc.SetLoad(entity.HexAddr, "0%.1"); err != nil {
			b.logger.Printf("avc set load off error: %v", err)
		}
		return
	}

	if cmd.Brightness != nil {
		// Explicit brightness — set directly
		value := fmt.Sprintf("%d%%.0", *cmd.Brightness)
		if err := avc.SetLoad(entity.HexAddr, value); err != nil {
			b.logger.Printf("avc set load error: %v", err)
		}
		return
	}

	if cmd.State != nil && *cmd.State == "ON" {
		// ON with no brightness
		if entity.FollowDaylight && entity.DaylightToggleAddr >= 0 {
			// Simulate natural light toggle button press
			if err := avc.SimulateButtonPress(entity.DaylightToggleAddr); err != nil {
				b.logger.Printf("avc button press error: %v", err)
			}
		} else {
			// Non-daylight load: set to max
			value := fmt.Sprintf("%d%%.0", entity.Max)
			if err := avc.SetLoad(entity.HexAddr, value); err != nil {
				b.logger.Printf("avc set load error: %v", err)
			}
		}
	}
}

// publishAllStates publishes the current state for every entity to MQTT.
func (b *Bridge) publishAllStates() {
	b.mu.RLock()
	defer b.mu.RUnlock()

	for i := range b.entities {
		e := &b.entities[i]
		st := b.state[e.UniqueID]
		if err := b.mqtt.PublishState(e, *st); err != nil {
			b.logger.Printf("publish state error for %s: %v", e.UniqueID, err)
		}
	}
	b.logger.Printf("published state for %d entities", len(b.entities))
}
