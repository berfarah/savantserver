package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// SavantAPI is an HTTP client for Savant's openapi-go REST API (default port 3062).
// openapi-go provides read-only access to configuration (rooms, loads, devices,
// scenes) and state values. It does NOT support hardware control — use AVCClient
// for that. See docs/02-rest-api.md for endpoint details.
type SavantAPI struct {
	baseURL string
	client  *http.Client
}

// NewSavantAPI creates a new SavantAPI client pointing at the given host and port.
func NewSavantAPI(host string, port int) *SavantAPI {
	return &SavantAPI{
		baseURL: fmt.Sprintf("http://%s:%d", host, port),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// FetchRooms retrieves all rooms from the Savant REST API.
func (s *SavantAPI) FetchRooms(ctx context.Context) ([]Room, error) {
	var rooms []Room
	if err := s.getJSON(ctx, "/config/v1/rooms", &rooms); err != nil {
		return nil, fmt.Errorf("fetching rooms: %w", err)
	}
	return rooms, nil
}

// FetchLoads retrieves all lighting loads from the Savant REST API.
func (s *SavantAPI) FetchLoads(ctx context.Context) ([]Load, error) {
	var loads []Load
	if err := s.getJSON(ctx, "/config/v1/lighting/loads", &loads); err != nil {
		return nil, fmt.Errorf("fetching loads: %w", err)
	}
	return loads, nil
}

// FetchDevices retrieves all lighting devices from the Savant REST API.
// Note: the list endpoint does not include loads or buttons arrays.
func (s *SavantAPI) FetchDevices(ctx context.Context) ([]Device, error) {
	var devices []Device
	if err := s.getJSON(ctx, "/config/v1/lighting/devices", &devices); err != nil {
		return nil, fmt.Errorf("fetching devices: %w", err)
	}
	return devices, nil
}

// FetchDevice retrieves a single device by ID, including its loads and buttons.
func (s *SavantAPI) FetchDevice(ctx context.Context, deviceID string) (*Device, error) {
	var device Device
	path := fmt.Sprintf("/config/v1/lighting/devices/%s", deviceID)
	if err := s.getJSON(ctx, path, &device); err != nil {
		return nil, fmt.Errorf("fetching device %s: %w", deviceID, err)
	}
	return &device, nil
}

// FetchState reads a single state value by name.
func (s *SavantAPI) FetchState(ctx context.Context, stateName string) (string, error) {
	var resp struct {
		Data []string `json:"data"`
	}
	path := fmt.Sprintf("/states/%s", stateName)
	if err := s.getJSON(ctx, path, &resp); err != nil {
		return "", fmt.Errorf("fetching state %s: %w", stateName, err)
	}
	if len(resp.Data) == 0 {
		return "", nil
	}
	return resp.Data[0], nil
}

// FetchStates reads multiple state values concurrently.
// Returns a map from state name to value.
func (s *SavantAPI) FetchStates(ctx context.Context, stateNames []string) (map[string]string, error) {
	type result struct {
		name  string
		value string
		err   error
	}

	results := make(chan result, len(stateNames))
	var wg sync.WaitGroup

	for _, name := range stateNames {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			val, err := s.FetchState(ctx, n)
			results <- result{name: n, value: val, err: err}
		}(name)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	states := make(map[string]string, len(stateNames))
	var errs []string
	for r := range results {
		if r.err != nil {
			errs = append(errs, r.err.Error())
			continue
		}
		states[r.name] = r.value
	}

	if len(errs) > 0 {
		return states, fmt.Errorf("errors fetching states: %s", strings.Join(errs, "; "))
	}
	return states, nil
}

// getJSON performs a GET request and decodes the JSON response into dest.
func (s *SavantAPI) getJSON(ctx context.Context, path string, dest interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, path)
	}

	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

// BuildEntities transforms rooms, loads, and devices into LightEntity values
// suitable for Home Assistant MQTT discovery.
//
// The devices slice may include devices with populated Buttons (from per-device
// FetchDevice calls). natural light toggle mapping is derived from buttons with
// "naturallighttogglewithoff" or "naturallighttoggle" functions.
func BuildEntities(rooms []Room, loads []Load, devices []Device) ([]LightEntity, error) {
	// 1. roomID → Room
	roomMap := make(map[string]Room, len(rooms))
	for _, r := range rooms {
		roomMap[r.RoomID] = r
	}

	// 2. deviceID → Device
	deviceMap := make(map[string]Device, len(devices))
	for _, d := range devices {
		deviceMap[d.DeviceID] = d
	}

	// 3. Build natural light toggle address map: roomID → switch address
	// Iterate devices with buttons to find natural light toggle buttons.
	daylightToggleMap := make(map[string]int)
	for _, d := range devices {
		if len(d.Buttons) == 0 {
			continue
		}
		devAddr, err := ParseDeviceAddress(d.Address)
		if err != nil {
			continue
		}
		for _, btn := range d.Buttons {
			fn := btn.Function
			if fn != "naturallighttogglewithoff" && fn != "naturallighttoggle" {
				continue
			}
			targetRoomID := btn.RoomID
			if targetRoomID == "" {
				continue
			}
			switchAddr := ComputeSwitchAddr(devAddr, btn.Index)
			daylightToggleMap[targetRoomID] = switchAddr
		}
	}

	// 4. Build entities from loads
	entities := make([]LightEntity, 0, len(loads))
	for _, load := range loads {
		dev, ok := deviceMap[load.DeviceID]
		if !ok {
			return nil, fmt.Errorf("device %s not found for load %s", load.DeviceID, load.LoadID)
		}

		room, ok := roomMap[load.RoomID]
		if !ok {
			return nil, fmt.Errorf("room %s not found for load %s", load.RoomID, load.LoadID)
		}

		devAddr, err := ParseDeviceAddress(dev.Address)
		if err != nil {
			return nil, fmt.Errorf("parsing device address for load %s: %w", load.LoadID, err)
		}

		hexAddr := ComputeHexAddr(devAddr, load.Offset)

		daylightToggleAddr := -1
		if load.FollowDaylight {
			if addr, ok := daylightToggleMap[load.RoomID]; ok {
				daylightToggleAddr = addr
			}
		}

		entity := LightEntity{
			UniqueID:       fmt.Sprintf("savant_load_%s_%d", dev.Address, load.Offset),
			Name:           load.Name,
			RoomName:       room.Name,
			RoomSlug:       Slugify(room.Name),
			LoadSlug:       Slugify(load.Name),
			DeviceAddress:  dev.Address,
			LoadOffset:     load.Offset,
			HexAddr:        hexAddr,
			IsDimmable:     load.Type == 0,
			FollowDaylight: load.FollowDaylight,
			Min:            load.Min,
			Max:            load.Max,
			DaylightToggleAddr:   daylightToggleAddr,
			DeviceModel:    dev.RPMLightingDeviceName,
		}
		entities = append(entities, entity)
	}

	return entities, nil
}
